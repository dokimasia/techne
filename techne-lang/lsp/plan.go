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
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Plan computes the edits an operation needs, and writes nothing.
//
// # Three operations, each with a request behind it
//
// Renaming a declaration is the one the protocol defines outright:
// textDocument/rename returns a workspace edit covering every file the
// name reaches, computed by the same type checker that resolves it.
// Moving a file is workspace/willRenameFiles, which is what an editor
// sends before it moves one and what mends the imports. Lifting a run of
// lines into a function is a code action, which is how every editor
// offers it.
//
// The rest of the catalogue is declined rather than approximated. What
// is common to the three is that the server computes the edits: a plan
// built here out of matched text would be wrong in exactly the cases
// nobody checks.
//
// # Nothing here touches disk
//
// The workspace edit is converted to [edit.Change] and handed back. It
// is techne's write path that reads the files, gates the result and
// applies it atomically — the same path a parser's plan goes through, so
// a server cannot weaken it. That is also why [answers.ApplyEdit]
// refuses unless techne asked for the edit itself.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	switch op {
	case edit.RenameSymbol:
		return e.renaming(ctx, req, target, args)
	case edit.MoveFile:
		return e.relocating(ctx, target, args)
	case edit.ExtractFunction:
		return e.extracting(ctx, req, target, args)
	}
	return engine.Result[edit.Change]{}, fmt.Errorf(
		"%w: %s: no request behind %s", engine.ErrDecline, e.server.Name, op)
}

// reached is what a plan's coverage is worth.
//
// A plan that rewrites the code referring to its target is a claim that
// every reference was found, and the policy admits one only over total
// coverage. A server that computed something has shown it looked, and
// that carries the claim. A server that computed nothing has not: an
// empty answer is what both a file nothing refers to and a server with
// no view of the workspace produce, and the two are indistinguishable
// in the answer itself. So an empty one falls back to the evidence
// every empty semantic answer rests on.
//
// metals is why: it answers a move with no edits and a rename over a
// build it has not imported with the declaration alone, and reports no
// progress to say it was not ready, so waiting on it settles nothing.
func (e *Engine) reached(
	ctx context.Context,
	held *session,
	p source.Path,
	shown bool,
) (trust.Completeness, []trust.Caveat) {
	covered, caveats := e.settled(ctx)
	if covered == trust.ScopeTotal && !shown && !e.analysed(ctx, held, p) {
		return trust.ScopePartial, append(caveats, unresolved)
	}
	return covered, caveats
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

// changes turns a workspace edit into what the write path applies.
func (e *Engine) changes(held *protocol.WorkspaceEdit) ([]edit.Change, error) {
	return e.changesAgainst(held, nil)
}

// changesAgainst is the same against text the server holds that is not
// what is on disk.
//
// A range is a line and a character count, and turning it into a byte
// offset needs the text the server counted in. That is the file for
// every operation but one: an extraction is renamed after it is made,
// and the buffer the rename was computed against is the extraction's
// result, which nothing has written yet. Converted against the file the
// rename would write over the wrong bytes.
//
// Both shapes are read. The older one is a map of files to edits; the
// newer one is an ordered list that may also create, rename and delete
// files, which a rename needs when a language ties a file's name to what
// it declares. A server sends one or the other, and reading only the map
// silently drops every file operation a rename implied.
func (e *Engine) changesAgainst(
	held *protocol.WorkspaceEdit,
	texts map[source.Path]document,
) ([]edit.Change, error) {
	if held == nil {
		return nil, nil
	}

	var out []edit.Change
	for held, edits := range held.Changes {
		change, err := e.rewriteAgainst(held, edits, texts)
		if err != nil {
			return nil, err
		}
		out = append(out, change)
	}

	for _, one := range held.DocumentChanges {
		change, carries, err := e.operation(one, texts)
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
func (e *Engine) operation(
	held protocol.DocumentChange,
	texts map[source.Path]document,
) (edit.Change, bool, error) {
	switch one := held.(type) {
	case *protocol.TextDocumentEdit:
		change, err := e.rewriteAgainst(one.TextDocument.URI, plain(one.Edits), texts)
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

// rewriteAgainst converts one file's edits into the coordinates this
// vocabulary counts in, counting in the text the server held.
//
// Sorted by where they start and checked for overlap, because the write
// path applies them in one pass and relies on both. A server is not
// required to send them in order.
func (e *Engine) rewriteAgainst(
	held uri.URI,
	edits []protocol.TextEdit,
	texts map[source.Path]document,
) (edit.Change, error) {
	p := e.pathOf(held)
	doc, given := texts[p]
	if !given {
		var err error
		if doc, err = e.read(p); err != nil {
			return edit.Change{}, err
		}
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
