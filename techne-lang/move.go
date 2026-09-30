// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// Moving returns the workspace paths of a move of a file, [edit.MoveFile]: the file that target
// names and the destination in args. root is the directory of the workspace. It returns an empty
// source for a file whose extension is not one of extensions, which the engine of another
// language moves.
//
// It returns an error that wraps [engine.ErrRefuse] for a target that is not a file, a missing
// destination, a move onto the file itself or across the edge of the workspace, a file that
// does not exist, and a destination where a file exists.
func Moving(root string, target edit.Target, args edit.Args, extensions []string) (source.Path, source.Path, error) {
	from := target.Path
	if target.Kind != edit.TargetFile || from == "" {
		return "", "", fmt.Errorf("%w: %s names a file, and this target names none", engine.ErrRefuse, edit.MoveFile)
	}
	to := source.Path(strings.TrimSpace(args[edit.ArgDestination]))
	switch {
	case to == "":
		return "", "", fmt.Errorf("%w: %s needs %s", engine.ErrRefuse, edit.MoveFile, edit.ArgDestination)
	case to == from:
		return "", "", fmt.Errorf("%w: %s is already at %s", engine.ErrRefuse, from, to)
	case Outside(from), Outside(to):
		return "", "", fmt.Errorf("%w: %s moves a file within the workspace, and %s is outside it",
			engine.ErrRefuse, edit.MoveFile, outsider(from, to))
	case !Claims(string(from), extensions):
		return "", "", nil
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(string(from)))); err != nil {
		return "", "", fmt.Errorf("%w: %s does not exist", engine.ErrRefuse, from)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(string(to)))); err == nil {
		return "", "", fmt.Errorf("%w: %s exists, and a move does not overwrite a file", engine.ErrRefuse, to)
	}
	return from, to, nil
}

// Outside reports whether p is outside the workspace: an absolute path, or a relative path
// whose first segment is "..". A name that starts with two dots, such as ..config.ts, is inside.
func Outside(p source.Path) bool {
	native := filepath.Clean(filepath.FromSlash(string(p)))
	return filepath.IsAbs(native) || native == ".." ||
		strings.HasPrefix(native, ".."+string(filepath.Separator))
}

// outsider returns the end of a move that is outside the workspace: from when from is outside,
// and to otherwise.
func outsider(from, to source.Path) source.Path {
	if Outside(from) {
		return from
	}
	return to
}
