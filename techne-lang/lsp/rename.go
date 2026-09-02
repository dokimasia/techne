// Copyright ThesmOS B.V. 2026
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
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// renaming rewrites a declaration's name and every use of it.
func (e *Engine) renaming(
	ctx context.Context,
	req engine.Request,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	const op = edit.RenameSymbol
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

	at, doc, known, err := e.aimed(ctx, held, req, target)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if !known {
		return engine.Result[edit.Change]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	// A plan computed against a half-loaded workspace rewrites the
	// references the server had found so far and leaves the rest, which
	// is the one outcome worse than refusing. Waited on after the files
	// are open, because opening them is what starts the work.
	e.working.settle(ctx, e.settling())

	// Who uses it, which opens the files holding the uses and is what
	// the rename below is measured against.
	uses := e.using(ctx, held, doc, at)

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
	covered, caveats := e.corroborated(ctx, held, doc, uses, changes)
	return engine.Result[edit.Change]{
		Items:        changes,
		Completeness: covered,
		Caveats:      caveats,
	}, nil
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
		if inside, known := covering(symbols, target.Span.Start.Offset); known {
			return naming(doc, inside), doc, true, nil
		}
		// A span covering no declaration is believed as it stands.
		// Whoever sent it may be pointing at something an outline does
		// not report, and a server is a better judge of that than this.
		return doc.mark(target.Span.Start), doc, true, nil

	case edit.TargetSymbol:
		subject, doc, known, read, err := e.declaring(ctx, held, req, target.Symbol)
		if err != nil || !read {
			return protocol.Position{}, document{}, false, err
		}
		if !known {
			// Read the files and found no such declaration. A plan
			// computed from a position nothing was found at rewrites
			// whatever happens to be there.
			return protocol.Position{}, document{}, false, fmt.Errorf(
				"%w: %s: no declaration in %q matches %s",
				engine.ErrRefuse, e.server.Name, req.Scope, target.Symbol)
		}
		return naming(doc, subject), doc, true, nil
	}

	return protocol.Position{}, document{}, false, fmt.Errorf(
		"%w: %s is pointed at nothing this engine can place", engine.ErrRefuse, e.server.Name)
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

// using is where the server says a declaration is used, and opens every
// file it names.
//
// # Opening them is the point as much as knowing them
//
// A server computes a rename over the buffers the client is holding.
// metals does exactly that and nothing more: asked to rename a class
// with only the class's own file open, it rewrites that file, renames it
// to match, and leaves every other use of the class where it was. It
// answers who uses the class perfectly well while doing so, because the
// index and the refactoring are not the same machinery.
//
// So the uses are asked for first and the files holding them are opened,
// which is the state an editor would have been in.
//
// # And they are the evidence
//
// A rename that rewrites references claims every reference was found.
// The plan alone cannot support that: a server with no compiler view
// answers with the declaration and nothing else, which is exactly what a
// declaration nothing uses looks like. The reference list is the second
// answer that claim is measured against.
func (e *Engine) using(
	ctx context.Context,
	held *session,
	doc document,
	at protocol.Position,
) []protocol.Location {
	if !provides(held.capable.ReferencesProvider) {
		return nil
	}
	pick := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     at,
	}
	answered, err := held.asks.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: pick,
		// The declaration itself is not a use, and a rename that
		// rewrote nothing but its own name would look complete.
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		// A server that will not answer this is one whose rename cannot
		// be corroborated. That is what the answer says, rather than a
		// reason to fail a rename the server would have done.
		return nil
	}

	for _, one := range answered {
		p := e.pathOf(one.URI)
		if outside(p) {
			// A server indexes what its own configuration covers, which
			// includes a standard library and a module cache. Those are
			// not techne's to open and not its to rewrite.
			continue
		}
		// A file that cannot be read is one the server named and this
		// cannot show it. Skipped rather than raised: the rename is
		// still worth doing, and the coverage below says what it is
		// worth.
		_ = e.open(ctx, held, p)
	}
	return answered
}

// corroborated is what a rename's coverage is worth, measured against
// what the server itself says uses the declaration.
//
// A rename covering every use the server can name is as complete as the
// server is. One that misses a use is short whatever it claims, and a
// caller acting on total coverage would delete the declaration.
//
// Where there is nothing to measure against — a server that answers no
// references, or a declaration nothing uses — it falls back to the
// evidence every empty semantic answer rests on: whether the server
// produced a view of the file at all.
func (e *Engine) corroborated(
	ctx context.Context,
	held *session,
	doc document,
	uses []protocol.Location,
	changes []edit.Change,
) (trust.Completeness, []trust.Caveat) {
	covered, caveats := e.settled(ctx)
	if covered != trust.ScopeTotal {
		return covered, caveats
	}
	if len(uses) > 0 {
		if missed, short := e.uncovered(uses, changes); short {
			return trust.ScopePartial, append(caveats, trust.Caveat{
				Code: trust.CaveatIndexWarming,
				Note: "the server names a use at " + missed + " that this change does not " +
					"rewrite, so it is not every use",
			})
		}
		return covered, caveats
	}
	if !e.analysed(ctx, held, doc.path) {
		return trust.ScopePartial, append(caveats, unresolved)
	}
	return covered, caveats
}

// uncovered names the first use a plan leaves alone, and reports whether
// there was one.
func (e *Engine) uncovered(uses []protocol.Location, changes []edit.Change) (string, bool) {
	edits := map[source.Path][]edit.TextEdit{}
	for _, c := range changes {
		if c.Kind == edit.ChangeEdit {
			edits[c.Path] = append(edits[c.Path], c.Edits...)
		}
	}

	// One read per file rather than one per use: a declaration used
	// thirty times in one file is one file.
	read := map[source.Path]document{}
	for _, one := range uses {
		p := e.pathOf(one.URI)
		if outside(p) {
			continue
		}
		doc, loaded := read[p]
		if !loaded {
			held, err := e.read(p)
			if err != nil {
				continue
			}
			doc, read[p] = held, held
		}
		if !rewrites(edits[p], doc.position(one.Range.Start).Offset) {
			return fmt.Sprintf("%s:%d", p, one.Range.Start.Line+1), true
		}
	}
	return "", false
}

// rewrites reports whether an edit list covers the byte at an offset.
func rewrites(edits []edit.TextEdit, at int) bool {
	for _, one := range edits {
		if one.Span.Start.Offset <= at && at < one.Span.End.Offset {
			return true
		}
	}
	return false
}
