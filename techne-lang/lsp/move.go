// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// relocating plans [edit.MoveFile] with workspace/willRenameFiles.
//
// A server returns the edits that update the references to the moved file, such as import
// specifiers in TypeScript and the class name in Java, and never the move itself. The plan
// contains those edits followed by the move. It returns [engine.ErrRefuse] for a move out of
// the workspace, onto an existing file, or of a file that does not exist, and a skipped result
// for a file of another language.
func (e *Engine) relocating(
	ctx context.Context,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	from := target.Path
	if target.Kind != edit.TargetFile || from == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s names a file, and this target names none", engine.ErrRefuse, edit.MoveFile)
	}
	to := source.Path(strings.TrimSpace(args[edit.ArgDestination]))
	switch {
	case to == "":
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s needs %s", engine.ErrRefuse, edit.MoveFile, edit.ArgDestination)
	case to == from:
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s is already at %s", engine.ErrRefuse, from, to)
	case outside(from), outside(to):
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s moves a file within the workspace, and %s is outside it",
			engine.ErrRefuse, edit.MoveFile, outsider(from, to))
	case !lang.Claims(string(from), e.declared.Extensions):
		return engine.Result[edit.Change]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}
	if _, err := os.Stat(e.fullPath(from)); err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s does not exist", engine.ErrRefuse, from)
	}
	if _, err := os.Stat(e.fullPath(to)); err == nil {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s exists, and a move does not overwrite a file", engine.ErrRefuse, to)
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()
	if !willRename(held.capable) {
		return engine.Result[edit.Change]{}, e.unsupported("workspace/willRenameFiles")
	}
	// The open file makes the server load the project that contains it. An import of the file
	// writes its stem.
	if _, err = e.open(ctx, held, from); err != nil {
		return engine.Result[edit.Change]{}, err
	}
	stem := strings.TrimSuffix(path.Base(string(from)), path.Ext(string(from)))
	short, err := e.preload(ctx, held, stem, from)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	ready := e.settle(ctx, held)

	answered, err := held.asks.WillRenameFiles(ctx, &protocol.RenameFilesParams{
		Files: []protocol.FileRename{{
			OldURI: string(uri.File(e.fullPath(from))),
			NewURI: string(uri.File(e.fullPath(to))),
		}},
	})
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: willRenameFiles: %w", engine.ErrDecline, e.server.Name, err)
	}
	changes, err := e.changes(answered, nil)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if reached := beyond(changes); reached != "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: the move changes %s, which is outside the workspace",
			engine.ErrRefuse, e.server.Name, reached)
	}

	// The edits apply before the move, so a server that renames the declaration inside the
	// moved file, as jdtls does, edits the file at its old path. An error on a line that writes
	// the stem outside the edits can hide an import.
	shown := len(changes) > 0
	risky := lang.Writing(stem, edited(changes))
	changes = append(changes, edit.Change{Kind: edit.ChangeMove, Path: from, To: to})
	covered, reaches, caveats := e.reached(ctx, held, from, shown, ready, risky)
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

// outside reports whether p is outside the workspace: an absolute path, which
// [Engine.pathOf] returns for a file outside the root, or a relative path whose first segment
// is "..". A name that starts with two dots, such as ..config.ts, is inside.
func outside(p source.Path) bool {
	native := filepath.Clean(filepath.FromSlash(string(p)))
	return filepath.IsAbs(native) || native == ".." ||
		strings.HasPrefix(native, ".."+string(filepath.Separator))
}

// outsider returns the end of a move that is outside the workspace: from when from is
// outside, and to otherwise.
func outsider(from, to source.Path) source.Path {
	if outside(from) {
		return from
	}
	return to
}
