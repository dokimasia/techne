// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// relocating moves a file and mends whatever named it.
//
// # The server is asked before the move, not after
//
// workspace/willRenameFiles is what an editor sends while the user is
// still dragging the file, and the answer is the edit that keeps the
// workspace whole: the import specifiers in TypeScript, the module path
// in Rust, the class name in Java, which ties a file's name to what it
// declares. Sent afterwards there is nothing left to compute it from,
// because the file the references point at is gone.
//
// # The move itself is techne's
//
// The server answers with the edits the move implies and never with the
// move. So the change list carries both: whatever the server said, and
// the relocation, which the write path performs. A server that answers
// nothing has said the move implies no edits, which for a file nothing
// imports is the right answer and not a missing one.
func (e *Engine) relocating(
	ctx context.Context,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	const op = edit.MoveFile

	from := target.Path
	if target.Kind != edit.TargetFile || from == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s names no file to move", engine.ErrRefuse, op)
	}
	to := source.Path(strings.TrimSpace(args[edit.ArgDestination]))
	switch {
	case to == "":
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s needs %s", engine.ErrRefuse, op, edit.ArgDestination)
	case to == from:
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s is already there", engine.ErrRefuse, from)
	case outside(from), outside(to):
		// A move whose ends are not both under the root cannot be applied
		// as described: techne writes under its workspace and reports
		// relative to it.
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s moves within the workspace, and %s is outside it",
			engine.ErrRefuse, op, outsider(from, to))
	}
	if _, err := os.Stat(e.fullPath(from)); err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: there is no %s to move", engine.ErrRefuse, from)
	}
	if _, err := os.Stat(e.fullPath(to)); err == nil {
		// Refused rather than overwritten. A move onto an existing file
		// is a file lost, and the write path would report it as one path
		// changed.
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s is already there, and a move does not overwrite",
			engine.ErrRefuse, to)
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !willRename(held.capable) {
		return engine.Result[edit.Change]{}, e.unsupported("workspace/willRenameFiles")
	}

	// Opening the file is what has the server load the project holding
	// it, and the settle is waited on after that for the same reason a
	// rename waits: an edit computed against a half-loaded workspace
	// mends the references found so far and leaves the rest.
	if opened := e.open(ctx, held, from); opened != nil {
		return engine.Result[edit.Change]{}, opened
	}
	e.working.settle(ctx, e.settling())

	answered, err := held.asks.WillRenameFiles(ctx, &protocol.RenameFilesParams{
		Files: []protocol.FileRename{{
			OldURI: string(uri.File(e.fullPath(from))),
			NewURI: string(uri.File(e.fullPath(to))),
		}},
	})
	if err != nil {
		// A server may declare the request and refuse this file, which is
		// a decline rather than a fault: a parser beside it gets a turn
		// and the caller is told what the server said.
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: will rename files: %w", engine.ErrDecline, e.server.Name, err)
	}

	changes, err := e.changes(answered)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if beyond := beyond(changes); beyond != "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: the move reaches %s, which is outside the workspace",
			engine.ErrRefuse, e.server.Name, beyond)
	}

	// Whether the server computed anything, read before the relocation
	// is added: a move it worked edits out for is one it looked at, and
	// a move that implies none is not.
	shown := len(changes) > 0

	// The relocation last, so the edits the server computed are applied
	// to the file before it goes. A server that renames the declaration
	// inside the file it moves — which is what Java means by moving a
	// file — sends exactly that.
	changes = append(changes, edit.Change{Kind: edit.ChangeMove, Path: from, To: to})

	covered, reaches, caveats := e.reached(ctx, held, from, shown)
	return engine.Result[edit.Change]{
		Items:        changes,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// willRename reports whether a server asked to be consulted before a
// file moves.
//
// Read from what it declared rather than found out by asking. gopls
// refuses the request with an error, which is indistinguishable from the
// move implying no edits — and reported as none it would move a file and
// leave every reference to it pointing nowhere.
func willRename(held protocol.ServerCapabilities) bool {
	return held.Workspace != nil &&
		held.Workspace.FileOperations != nil &&
		len(held.Workspace.FileOperations.WillRename.Filters) > 0
}

// outside reports whether a path leaves the workspace.
//
// An absolute path is one [Engine.pathOf] could not make relative, and a
// relative one climbing out of the root is the same thing spelt
// differently. The separator is part of the test: a file called
// ..config.ts starts with two dots and is inside the workspace.
func outside(p source.Path) bool {
	held := filepath.Clean(filepath.FromSlash(string(p)))
	return filepath.IsAbs(held) ||
		held == ".." || strings.HasPrefix(held, ".."+string(filepath.Separator))
}

// outsider names whichever end of a move is not under the root.
func outsider(from, to source.Path) source.Path {
	if outside(from) {
		return from
	}
	return to
}
