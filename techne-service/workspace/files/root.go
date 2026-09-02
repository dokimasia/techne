// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"go.dokimi.dev/techne/core/source"
)

// Root is one directory, opened for reading and for writing.
//
// It is safe for concurrent use. Serialising two changes to one file is
// the write path's job, and is a different question from whether two
// goroutines may call this at once.
type Root struct{ root *os.Root }

// Open opens a directory. The caller closes it when finished.
func Open(dir string) (*Root, error) {
	held, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("files: open %q: %w", dir, err)
	}
	return &Root{root: held}, nil
}

// Close releases the directory.
func (r *Root) Close() error { return r.root.Close() }

// FS returns the reading half, which is what an engine is handed.
func (r *Root) FS() fs.FS { return r.root.FS() }

// Read returns a file's content.
func (r *Root) Read(p source.Path) ([]byte, error) {
	return r.root.ReadFile(string(p))
}

// Write replaces a file's content, keeping the mode it had.
//
// The bytes are staged beside the target and renamed over it, so a
// process that stops partway leaves the original rather than half of
// each. A rename within one directory is atomic on every filesystem
// techne runs on.
//
// A file that does not exist yet is created at 0644 along with whatever
// directories it needs, because a change that makes a file in a package
// that does not exist yet is a change, not a failure.
func (r *Root) Write(p source.Path, content []byte) error {
	mode, err := r.prepare(p)
	if err != nil {
		return err
	}

	staged := string(p) + Partial
	if err := r.root.WriteFile(staged, content, mode); err != nil {
		return fmt.Errorf("files: stage %s: %w", p, err)
	}
	if err := r.root.Rename(staged, string(p)); err != nil {
		// The staged file is this call's litter, and leaving it would
		// make the next call fail on a name it does not own.
		return errors.Join(
			fmt.Errorf("files: write %s: %w", p, err),
			r.root.Remove(staged),
		)
	}
	return nil
}

// prepare returns the mode to write a file at, making its directories
// where it is new.
func (r *Root) prepare(p source.Path) (fs.FileMode, error) {
	switch info, err := r.root.Stat(string(p)); {
	case err == nil:
		// A file that was executable stays executable. Rewriting it at
		// the default would break whatever ran it.
		return info.Mode().Perm(), nil
	case !errors.Is(err, fs.ErrNotExist):
		return 0, fmt.Errorf("files: stat %s: %w", p, err)
	}

	if dir := path.Dir(string(p)); dir != "." {
		if err := r.root.MkdirAll(dir, 0o755); err != nil {
			return 0, fmt.Errorf("files: make %s: %w", dir, err)
		}
	}
	return 0o644, nil
}

// Remove takes a file away.
//
// A file that is already gone is not an error: the workspace is in the
// state the caller asked for, and a change that removes what a previous
// one removed should not fail on the second try.
func (r *Root) Remove(p source.Path) error {
	if err := r.root.Remove(string(p)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("files: remove %s: %w", p, err)
	}
	return nil
}

// Partial is the suffix a staged file carries.
//
// It is not any language's extension, so nothing routes to one a crash
// left behind, and it names techne so whoever finds one knows what put
// it there.
const Partial = ".techne-partial"
