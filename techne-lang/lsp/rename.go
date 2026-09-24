// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// renaming plans [edit.RenameSymbol] with textDocument/rename in these steps:
//
//  1. Open the file of the declaration, and wait for the server to settle.
//  2. Open every file that textDocument/references names, because metals renames only in its
//     open buffers.
//  3. Ask textDocument/prepareRename where the server offers it.
//  4. Ask textDocument/rename.
//
// renaming returns [engine.ErrRefuse] when the server refuses the position or the rename. A
// plan that leaves a use unrewritten is partial.
func (e *Engine) renaming(
	ctx context.Context,
	req engine.Request,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	fresh := strings.TrimSpace(args[edit.ArgNewName])
	if fresh == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s needs %s", engine.ErrRefuse, edit.RenameSymbol, edit.ArgNewName)
	}
	files, skipped, err := e.targeted(req, target)
	if err != nil || skipped {
		return engine.Result[edit.Change]{Skipped: skipped, Completeness: trust.ScopeTotal}, err
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()
	if !provides(held.capable.RenameProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/rename")
	}
	at, doc, err := e.aimed(ctx, held, req, target, files)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	short, err := e.preload(ctx, held, lang.WordAt(string(doc.content), doc.position(at).Offset), doc.path)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	ready := e.settle(ctx, held)
	uses := e.using(ctx, held, doc, at)

	if prepares(held.capable.RenameProvider) {
		prepared, refused := held.asks.PrepareRename(ctx, &protocol.PrepareRenameParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
			Position:     at,
		})
		if refused != nil {
			return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s: %s",
				engine.ErrRefuse, e.server.Name, reasoned(refused))
		}
		if prepared == nil {
			return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s: nothing at %s:%d:%d can be renamed",
				engine.ErrRefuse, e.server.Name, doc.path, at.Line+1, at.Character+1)
		}
	}

	answered, err := held.asks.Rename(ctx, &protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     at,
		NewName:      fresh,
	})
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s: %s", engine.ErrRefuse, e.server.Name, reasoned(err))
	}
	changes, err := e.changes(answered, nil)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if outside := beyond(changes); outside != "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: the rename changes %s, which is outside the workspace",
			engine.ErrRefuse, e.server.Name, outside)
	}
	covered, reaches, caveats := e.corroborated(ctx, held, doc, at, uses, changes, ready)
	if short != nil {
		covered, caveats = trust.ScopePartial, append(caveats, *short)
	}
	return engine.Result[edit.Change]{
		Items:        changes,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// targeted returns the files of the scope of req for a target that names a declaration, and
// reports true for a target without a file of the language: a span in a file of another
// language, or a declaration in a scope without files of the language.
func (e *Engine) targeted(req engine.Request, target edit.Target) (lang.Files, bool, error) {
	switch target.Kind {
	case edit.TargetSpan:
		return lang.Files{}, !lang.Claims(string(target.Span.Path), e.declared.Extensions), nil
	case edit.TargetSymbol:
		files, err := e.walk(req)
		if err != nil {
			return lang.Files{}, false, err
		}
		return files, len(files.Read) == 0 && len(files.Unread) == 0, nil
	}
	return lang.Files{}, false, fmt.Errorf("%w: %s names a declaration or a span, and this target names neither",
		engine.ErrRefuse, edit.RenameSymbol)
}

// aimed opens the file of target and returns the protocol position of the name of the
// declaration that target names, and the document of the file.
//
// A span names the innermost declaration that contains its start, and a span outside every
// declaration names its own start. A declaration is looked up in the files of the walk, and a
// declaration that no file declares returns [engine.ErrRefuse].
func (e *Engine) aimed(
	ctx context.Context,
	held *session,
	req engine.Request,
	target edit.Target,
	files lang.Files,
) (protocol.Position, document, error) {
	if target.Kind == edit.TargetSymbol {
		subject, doc, known, err := e.declaring(ctx, newFinder(e, held), req, target.Symbol, files.Read)
		switch {
		case err != nil:
			return protocol.Position{}, document{}, err
		case !known:
			return protocol.Position{}, document{}, fmt.Errorf("%w: %s: no declaration in %s matches %s%s",
				engine.ErrRefuse, e.server.Name, req.Scope, target.Symbol, skipping(files.Unread))
		}
		return naming(doc, subject), doc, nil
	}

	doc, err := e.open(ctx, held, target.Span.Path)
	if err != nil {
		return protocol.Position{}, document{}, err
	}
	symbols, err := e.symbols(ctx, held, doc)
	if err != nil {
		return protocol.Position{}, document{}, err
	}
	start := doc.mark(target.Span.Start)
	if inside, known := innermost(symbols, doc.position(start).Offset); known {
		return naming(doc, inside), doc, nil
	}
	return start, doc, nil
}

// using returns the uses of the declaration at position at from textDocument/references,
// without the declaration, and opens every file in the workspace that contains one. It returns
// nil for a server that does not serve references or that refuses the request.
func (e *Engine) using(
	ctx context.Context,
	held *session,
	doc document,
	at protocol.Position,
) []protocol.Location {
	if !provides(held.capable.ReferencesProvider) {
		return nil
	}
	answered, err := held.asks.References(ctx, &protocol.ReferenceParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     at,
		Context:      protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		return nil
	}
	for _, one := range answered {
		if p := e.pathOf(one.URI); !outside(p) {
			_, _ = e.open(ctx, held, p)
		}
	}
	return answered
}

// corroborated returns the completeness, the lowered tier and the caveats of a rename at the
// position at of doc, measured against the uses the server named. A plan that rewrites every
// use is as complete as the server's answer, and a plan that leaves a use unrewritten is
// partial. Without uses to compare, a server that has not shown a view of the file makes the
// plan partial. An error lowers the plan when it is on a line that writes the old name outside
// the edits of the plan.
func (e *Engine) corroborated(
	ctx context.Context,
	held *session,
	doc document,
	at protocol.Position,
	uses []protocol.Location,
	changes []edit.Change,
	ready bool,
) (trust.Completeness, trust.Fidelity, []trust.Caveat) {
	risky := lang.Writing(replaced(doc, at, changes), edited(changes))
	covered, reaches, caveats := e.bound(held, doc.path, ready, risky)
	switch {
	case covered != trust.ScopeTotal:
		return covered, reaches, caveats
	case len(uses) > 0:
		if missed, short := e.uncovered(uses, changes); short {
			return trust.ScopePartial, reaches, append(caveats, trust.Caveat{
				Code: trust.CaveatUnrewritten,
				Note: "the server names a use at " + missed + " that the rename does not rewrite",
			})
		}
		return covered, reaches, caveats
	case !e.analysed(ctx, held, doc.path):
		return trust.ScopePartial, reaches, append(caveats, unresolved)
	}
	return covered, reaches, caveats
}

// uncovered returns the path and line of the first use in the workspace that no edit of
// changes rewrites, and reports whether there is one.
func (e *Engine) uncovered(uses []protocol.Location, changes []edit.Change) (string, bool) {
	edits := map[source.Path][]edit.TextEdit{}
	for _, c := range changes {
		if c.Kind == edit.ChangeEdit {
			edits[c.Path] = append(edits[c.Path], c.Edits...)
		}
	}
	docs := map[source.Path]document{}
	for _, one := range uses {
		p := e.pathOf(one.URI)
		if outside(p) {
			continue
		}
		doc, read := docs[p]
		if !read {
			loaded, err := e.read(p)
			if err != nil {
				continue
			}
			doc, docs[p] = loaded, loaded
		}
		if !rewrites(edits[p], doc.position(one.Range.Start).Offset) {
			return fmt.Sprintf("%s:%d", p, one.Range.Start.Line+1), true
		}
	}
	return "", false
}

// replaced returns the text that the edit of changes at the position at of doc replaces, which
// is the old name of a rename, or the empty string when no edit covers the position.
func replaced(doc document, at protocol.Position, changes []edit.Change) string {
	offset := doc.position(at).Offset
	for _, c := range changes {
		if c.Kind != edit.ChangeEdit || c.Path != doc.path {
			continue
		}
		for _, one := range c.Edits {
			if one.Span.Start.Offset <= offset && offset < one.Span.End.Offset {
				return doc.text(one.Span)
			}
		}
	}
	return ""
}

// edited returns the lines that the edits of changes replace.
func edited(changes []edit.Change) lang.Lines {
	var spans []source.Span
	for _, c := range changes {
		for _, one := range c.Edits {
			span := one.Span
			span.Path = c.Path
			spans = append(spans, span)
		}
	}
	return lang.Spanned(spans...)
}

// rewrites reports whether an edit of edits replaces the byte at offset.
func rewrites(edits []edit.TextEdit, offset int) bool {
	for _, one := range edits {
		if one.Span.Start.Offset <= offset && offset < one.Span.End.Offset {
			return true
		}
	}
	return false
}

// reasoned returns the message of a server's error response without the "jsonrpc2: " frame
// that the connection adds.
func reasoned(err error) string {
	message := err.Error()
	if _, after, cut := strings.Cut(message, ": "); cut && strings.HasPrefix(message, "jsonrpc2: ") {
		return after
	}
	return message
}
