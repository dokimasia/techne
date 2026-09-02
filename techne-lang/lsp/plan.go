// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Plan computes the edits an operation needs, and writes nothing.
//
// # One operation, for now
//
// Renaming a declaration is the operation the protocol defines outright:
// textDocument/rename returns a workspace edit covering every file the
// name reaches, computed by the same type checker that resolves it.
// Everything else in the catalogue is declined rather than approximated.
//
// # Nothing here touches disk
//
// The workspace edit is converted to [edit.Change] and handed back. It
// is techne's write path that reads the files, gates the result and
// applies it atomically — the same path a parser's plan goes through, so
// a server cannot weaken it. That is also why [answers.ApplyEdit]
// refuses: a server offering to write is offering to bypass all of it.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	if op != edit.RenameSymbol {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: no request behind %s", engine.ErrDecline, e.server.Name, op)
	}
	fresh := strings.TrimSpace(args[edit.ArgNewName])
	if fresh == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s needs %s", engine.ErrRefuse, op, edit.ArgNewName)
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	if !provides(held.capable.RenameProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/rename")
	}

	// A plan computed against a half-loaded workspace rewrites the
	// references the server had found so far and leaves the rest, which
	// is the one outcome worse than refusing.
	e.working.settle(ctx, e.settling())

	at, doc, known, err := e.aimed(ctx, held, req, target)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if !known {
		return engine.Result[edit.Change]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	// Asked first, where the server answers it: whether the thing at
	// this position can be renamed at all. Skipping it turns a keyword
	// or a literal into a rename that reports no edits and reads as a
	// rename that had nothing to do.
	//
	// Not every server answers it, and one that does not refuses with an
	// error indistinguishable from the position being unrenameable. So
	// it is asked only where it was offered, and the rename goes ahead
	// without it otherwise.
	if prepares(held.capable.RenameProvider) {
		ready, refused := held.asks.PrepareRename(ctx, &protocol.PrepareRenameParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
			Position:     at,
		})
		if refused != nil {
			return engine.Result[edit.Change]{}, fmt.Errorf("lsp: %s: prepare rename: %w",
				e.server.Name, refused)
		}
		if ready == nil {
			return engine.Result[edit.Change]{}, fmt.Errorf(
				"%w: %s: nothing at %s:%d:%d can be renamed",
				engine.ErrRefuse, e.server.Name, doc.path, at.Line+1, at.Character+1)
		}
	}

	answered, err := held.asks.Rename(ctx, &protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     at,
		NewName:      fresh,
	})
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("lsp: %s: rename: %w", e.server.Name, err)
	}

	changes, err := e.changes(answered)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if outside := beyond(changes); outside != "" {
		// A server indexes whatever its own configuration covers, which
		// for a multi-module workspace is more than techne was pointed
		// at. techne applies changes under its root and reports them
		// relative to it, so a plan reaching past it cannot be applied
		// as described — and half of a rename is worse than none.
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: the rename reaches %s, which is outside the workspace",
			engine.ErrRefuse, e.server.Name, outside)
	}
	covered, caveats := e.settled(ctx)
	return engine.Result[edit.Change]{
		Items:        changes,
		Completeness: covered,
		Caveats:      caveats,
	}, nil
}

// covering is the smallest declaration holding an offset.
func covering(held []sema.Symbol, offset int) (sema.Symbol, bool) {
	var found sema.Symbol
	var known bool
	for _, one := range held {
		if one.Span.Start.Offset > offset || one.Span.End.Offset < offset {
			continue
		}
		if !known || covers(found.Span) > covers(one.Span) {
			found, known = one, true
		}
	}
	return found, known
}

// beyond names the first change that falls outside the workspace, or
// the empty string when none does.
func beyond(held []edit.Change) string {
	for _, one := range held {
		for _, p := range []source.Path{one.Path, one.To} {
			if p != "" && filepath.IsAbs(filepath.FromSlash(string(p))) {
				return string(p)
			}
		}
	}
	return ""
}

// aimed is the position an operation is pointed at.
//
// Both kinds resolve to the same thing: the place a declaration's own
// name is written. A span covers the whole declaration and starts at
// whatever opens it — class, type, def — and a server asked about a
// keyword answers that there is nothing there to rename. So the span is
// used to find the declaration and the declaration to find its name.
func (e *Engine) aimed(
	ctx context.Context,
	held *session,
	req engine.Request,
	target edit.Target,
) (protocol.Position, document, bool, error) {
	switch target.Kind {
	case edit.TargetSpan:
		symbols, doc, err := e.symbols(ctx, held, target.Span.Path)
		if err != nil {
			return protocol.Position{}, document{}, false, err
		}
		// The innermost declaration covering the span, for the reason
		// [finder.at] takes the innermost: a span inside a method is
		// inside the type holding it, and the caller meant the one it
		// pointed at.
		if within, known := covering(symbols, target.Span.Start.Offset); known {
			return naming(doc, within), doc, true, nil
		}
		// A span covering no declaration is believed as it stands.
		// Whoever sent it may be pointing at something an outline does
		// not report, and a server is a better judge of that than this.
		return doc.mark(target.Span.Start), doc, true, nil

	case edit.TargetSymbol:
		subject, doc, known, err := e.declaring(ctx, held, req, target.Symbol)
		if err != nil || !known {
			return protocol.Position{}, document{}, false, err
		}
		return naming(doc, subject), doc, true, nil
	}

	return protocol.Position{}, document{}, false, fmt.Errorf(
		"%w: %s is pointed at nothing this engine can place", engine.ErrRefuse, e.server.Name)
}

// changes turns a workspace edit into what the write path applies.
//
// Both shapes are read. The older one is a map of files to edits; the
// newer one is an ordered list that may also create, rename and delete
// files, which a rename needs when a language ties a file's name to what
// it declares. A server sends one or the other, and reading only the map
// silently drops every file operation a rename implied.
func (e *Engine) changes(held *protocol.WorkspaceEdit) ([]edit.Change, error) {
	if held == nil {
		return nil, nil
	}

	var out []edit.Change
	for held, edits := range held.Changes {
		change, err := e.rewrite(held, edits)
		if err != nil {
			return nil, err
		}
		out = append(out, change)
	}

	for _, one := range held.DocumentChanges {
		change, carries, err := e.operation(one)
		if err != nil {
			return nil, err
		}
		if carries {
			out = append(out, change)
		}
	}

	// A map has no order and the write path reports what it did, so the
	// same rename must not describe itself differently twice.
	slices.SortFunc(out, func(a, b edit.Change) int {
		return strings.Compare(string(a.Path), string(b.Path))
	})
	return out, nil
}

// operation reads one entry of the ordered shape.
func (e *Engine) operation(held protocol.DocumentChange) (edit.Change, bool, error) {
	switch one := held.(type) {
	case *protocol.TextDocumentEdit:
		change, err := e.rewrite(one.TextDocument.URI, plain(one.Edits))
		return change, true, err

	case *protocol.CreateFile:
		return edit.Change{Kind: edit.ChangeCreate, Path: e.pathOf(one.URI)}, true, nil

	case *protocol.RenameFile:
		return edit.Change{
			Kind: edit.ChangeMove,
			Path: e.pathOf(one.OldURI),
			To:   e.pathOf(one.NewURI),
		}, true, nil

	case *protocol.DeleteFile:
		return edit.Change{Kind: edit.ChangeDelete, Path: e.pathOf(one.URI)}, true, nil
	}
	return edit.Change{}, false, nil
}

// plain reads the edits out of whichever arm each one arrived in.
//
// An annotated edit is a plain edit plus a label for a user interface to
// group changes under, and a snippet edit carries placeholders an editor
// would let someone tab through. Neither is anything here can use, and
// both replace a range with text.
func plain(held []protocol.TextDocumentEditElement) []protocol.TextEdit {
	out := make([]protocol.TextEdit, 0, len(held))
	for _, one := range held {
		switch edited := one.(type) {
		case *protocol.TextEdit:
			out = append(out, *edited)
		case *protocol.AnnotatedTextEdit:
			out = append(out, protocol.TextEdit{Range: edited.Range, NewText: edited.NewText})
		case *protocol.SnippetTextEdit:
			// A snippet's text is a template rather than source, so
			// applying it would write the placeholders into the file.
			continue
		}
	}
	return out
}

// rewrite converts one file's edits into the coordinates this vocabulary
// counts in.
//
// Sorted by where they start and checked for overlap, because the write
// path applies them in one pass and relies on both. A server is not
// required to send them in order.
func (e *Engine) rewrite(held uri.URI, edits []protocol.TextEdit) (edit.Change, error) {
	p := e.pathOf(held)
	doc, err := e.read(p)
	if err != nil {
		return edit.Change{}, err
	}

	out := make([]edit.TextEdit, 0, len(edits))
	for _, one := range edits {
		out = append(out, edit.TextEdit{Span: doc.span(one.Range), New: one.NewText})
	}
	slices.SortFunc(out, func(a, b edit.TextEdit) int {
		return a.Span.Start.Offset - b.Span.Start.Offset
	})

	for i := 1; i < len(out); i++ {
		if out[i-1].Span.End.Offset > out[i].Span.Start.Offset {
			return edit.Change{}, fmt.Errorf(
				"lsp: %s: overlapping edits in %s at %d and %d",
				e.server.Name, p, out[i-1].Span.Start.Offset, out[i].Span.Start.Offset)
		}
	}
	return edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: out}, nil
}
