// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
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
//  3. Ask textDocument/prepareRename where the server offers it, at the declaration or else at
//     a use in its file, by the rule of [Engine.prepared]. A rename from a use takes the uses
//     that textDocument/references names from there when it named none from the declaration.
//  4. Ask textDocument/rename where the server prepared it.
//  5. Write out each edit at a shorthand property of an object literal, by the rule of
//     [Engine.shorthanded].
//
// renaming returns [engine.ErrRefuse] when the server refuses the position or the rename. A
// plan that leaves a use unrewritten is partial, and so are a plan that moves a file of a server
// that does not serve workspace/willRenameFiles, by the rule of [Engine.unmoved], and a plan
// with a shorthand property whose sides [Engine.shorthanded] cannot tell apart.
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
	defer e.reading()()
	ctx, done := e.answered(ctx)
	defer done()
	if !provides(held.capable.RenameProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/rename")
	}
	at, doc, err := e.aimed(ctx, held, req, target, files)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	short, _, err := e.preloaded(ctx, held, newFinder(e, held), doc, at)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	ready := e.settle(ctx, held)
	uses := e.using(ctx, held, doc, at)

	aim, found := at, true
	if prepares(held.capable.RenameProvider) {
		aim, found, err = e.prepared(ctx, held, doc, at)
		if err != nil {
			return engine.Result[edit.Change]{}, err
		}
	}
	switch {
	case !found:
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s: nothing at %s:%d:%d can be renamed",
			engine.ErrRefuse, e.server.Name, doc.path, at.Line+1, at.Character+1)
	case aim != at && len(uses) == 0:
		uses = e.using(ctx, held, doc, aim)
	}

	answered, err := held.asks.Rename(ctx, &protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     aim,
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
	unspelled, err := e.shorthanded(ctx, held, changes)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	covered, reaches, caveats := e.corroborated(ctx, held, doc, at, uses, changes, ready)
	if short != nil {
		covered, caveats = trust.ScopePartial, append(caveats, *short)
	}
	if unspelled != nil {
		covered, caveats = trust.ScopePartial, append(caveats, *unspelled)
	}
	if moved, unseen := e.unmoved(held, changes); unseen {
		covered, caveats = trust.ScopePartial, append(caveats, trust.Caveat{
			Code: trust.CaveatUnrewritten,
			Note: moved,
		})
	}
	return engine.Result[edit.Change]{
		Items:        changes,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// prepared returns the protocol position in doc from which the server prepares the rename of the
// declaration whose name is at the position at, and reports whether there is one. That is at,
// or else a use of the declaration in doc that [Engine.useIn] finds, for a server that prepares
// no rename at the declaration: ruby-lsp prepares the rename of a constant that a value assigns
// at each use of the constant and not at the assignment, and the rename from a use renames the
// assignment too. prepared returns the refusal of the server at the declaration as an error that
// wraps [engine.ErrRefuse].
func (e *Engine) prepared(
	ctx context.Context,
	held *session,
	doc document,
	at protocol.Position,
) (protocol.Position, bool, error) {
	document := protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))}
	prepared, refused := held.asks.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocument: document,
		Position:     at,
	})
	switch {
	case refused != nil:
		return protocol.Position{}, false, fmt.Errorf("%w: %s: %s", engine.ErrRefuse, e.server.Name, reasoned(refused))
	case prepared != nil:
		return at, true, nil
	}
	subject, known, err := newFinder(e, held).at(ctx, doc.path, at)
	if err != nil || !known {
		return protocol.Position{}, false, err
	}
	_, use, used, err := e.useIn(ctx, held, []source.Path{doc.path}, doc, subject)
	if err != nil || !used {
		return protocol.Position{}, false, err
	}
	prepared, refused = held.asks.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocument: document,
		Position:     use,
	})
	return use, refused == nil && prepared != nil, nil
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
	case edit.TargetUnset, edit.TargetFile:
	}
	return lang.Files{}, false, fmt.Errorf("%w: %s names a declaration or a span, and this target names neither",
		engine.ErrRefuse, edit.RenameSymbol)
}

// aimed opens the file of target and returns the protocol position of the name of the
// declaration that target names, and the document of the file.
//
// A span names the declaration of the outline engine of the language that [Engine.parsedAt]
// returns. A tool addresses a declaration by the span and the ID that the engine reports, and
// the engine reports the locals and the parameters that a server leaves out of its document
// symbols. Any other span names the innermost document symbol of the server that contains its
// start, and a span outside every symbol names its own start. A declaration is looked up in the
// files of the walk, and a declaration that no file declares returns [engine.ErrRefuse].
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
	parsed, known, err := e.parsedAt(ctx, held, target)
	switch {
	case err != nil:
		return protocol.Position{}, document{}, err
	case known:
		return naming(doc, parsed), doc, nil
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

// parsedAt returns the declaration of the outline engine of the language that target names, and
// reports whether there is one: the declaration whose span is the span of target, with the ID of
// target when target has one. The names that one declaration lists share its span, as the
// parameters have and want of func growCap(have, want int) do, and the ID selects one of them.
// An engine without an outline engine reports none.
func (e *Engine) parsedAt(ctx context.Context, held *session, target edit.Target) (sema.Symbol, bool, error) {
	if e.outliner == nil {
		return sema.Symbol{}, false, nil
	}
	span := target.Span
	kept, err := newFinder(e, held).file(ctx, span.Path)
	if err != nil {
		return sema.Symbol{}, false, err
	}
	for _, one := range kept.symbols {
		spanned := one.Span.Start.Offset == span.Start.Offset && one.Span.End.Offset == span.End.Offset
		if spanned && (target.Symbol == "" || one.ID == target.Symbol) {
			return one, true, nil
		}
	}
	return sema.Symbol{}, false, nil
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
		if p := e.pathOf(one.URI); !lang.Outside(p) {
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
		if missed, short := e.uncovered(uses, changes, doc.word(at)); short {
			return trust.ScopePartial, reaches, append(caveats, trust.Caveat{
				Code: trust.CaveatUnrewritten,
				Note: missed,
			})
		}
		return covered, reaches, caveats
	case !e.analysed(ctx, held, doc.path):
		return trust.ScopePartial, reaches, append(caveats, unresolved)
	}
	return covered, reaches, caveats
}

// uncovered returns the note of the caveat about the first use in the workspace that no edit of
// changes rewrites, and reports whether there is one. A plan edits no file that [lang.Readable]
// refuses, so each use in such a file is one: a use in a source that the build generates under
// a directory that .gitignore excludes, for example. A use in a file that cannot be read for
// another reason, such as a file that no longer exists, is left out, and so is a use whose text
// does not write name, the old name, as a word: csharp-ls reports the new of a target-typed
// new() as a use of its type, and a rename leaves the new as it is. An empty name leaves out no
// use.
func (e *Engine) uncovered(uses []protocol.Location, changes []edit.Change, name string) (string, bool) {
	edits := map[source.Path][]edit.TextEdit{}
	for _, c := range changes {
		if c.Kind == edit.ChangeEdit {
			edits[c.Path] = append(edits[c.Path], c.Edits...)
		}
	}
	docs := map[source.Path]document{}
	for _, one := range uses {
		p := e.pathOf(one.URI)
		if lang.Outside(p) {
			continue
		}
		at := fmt.Sprintf("%s:%d", p, one.Range.Start.Line+1)
		doc, read := docs[p]
		if !read {
			loaded, err := e.read(p)
			switch {
			case refused(err):
				return "the server names a use at " + at + " in a file that techne does not read", true
			case err != nil:
				continue
			}
			doc, docs[p] = loaded, loaded
		}
		span := doc.span(one.Range)
		named := name == "" || lang.Worded(doc.text(span), name) >= 0
		if named && !rewrites(edits[p], span) {
			return "the server names a use at " + at + " that the rename does not rewrite", true
		}
	}
	return "", false
}

// unmoved returns the note of the caveat about the first move of changes, and reports whether
// changes move a file, for a server that does not serve workspace/willRenameFiles. That request
// returns the edits of the paths that name a moved file, and no other request shows whether a
// rename rewrote them: ruby-lsp 0.26.11 moves the file of a class with the class, and leaves the
// require_relative that names the file. The rule leaves the rename of a server that serves the
// request as the server plans it.
func (e *Engine) unmoved(held *session, changes []edit.Change) (string, bool) {
	if willRename(held.capable) {
		return "", false
	}
	for _, c := range changes {
		if c.Kind == edit.ChangeMove {
			return fmt.Sprintf("the rename moves %s to %s, and %s does not serve "+
				"workspace/willRenameFiles, which rewrites the paths that name a moved file",
				c.Path, c.To, e.server.Name), true
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

// rewrites reports whether an edit of edits changes the text of use, the span of a use: whether
// the edit shares a byte with the span or inserts at one of its ends. A server can edit part of
// a use, and a rename is complete when an edit changes each use:
//
//   - jdtls reports the org.springframework.asm.ClassReader of a Javadoc link from character 38
//     to 73 of its line, and renames characters 62 to 73.
//   - csharp-ls renames EncodingHelper to EncodingHelperX by inserting X at the end of each use.
func rewrites(edits []edit.TextEdit, use source.Span) bool {
	for _, one := range edits {
		if one.Span.Start.Offset <= use.End.Offset && use.Start.Offset <= one.Span.End.Offset {
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
