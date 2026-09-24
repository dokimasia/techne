// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"go.dokimi.dev/techne/core/source"
)

// Root is one directory of a workspace, opened for reading and writing. It is safe for
// concurrent use. The write path serialises two changes of one file.
type Root struct {
	root *os.Root
	// dir is the absolute path of the directory, with its symbolic links resolved.
	dir string
}

// Open opens the directory at dir. The caller closes it.
func Open(dir string) (*Root, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		resolved, err = filepath.Abs(resolved)
	}
	if err != nil {
		return nil, fmt.Errorf("files: open %q: %w", dir, err)
	}
	opened, err := os.OpenRoot(resolved)
	if err != nil {
		return nil, fmt.Errorf("files: open %q: %w", dir, err)
	}
	return &Root{root: opened, dir: resolved}, nil
}

// Close releases the directory.
func (r *Root) Close() error { return r.root.Close() }

// FS returns the directory as an [fs.FS], which the engines read.
func (r *Root) FS() fs.FS { return r.root.FS() }

// Read returns the content of the file at p, and an error that wraps [fs.ErrNotExist] for
// a path without a file.
func (r *Root) Read(p source.Path) ([]byte, error) {
	return r.root.ReadFile(string(p))
}

// Write replaces the content of the file at p and keeps its mode. The content is staged
// beside the file and renamed over it, so a process that stops leaves the original file. A
// symbolic link inside the workspace is followed: the file that it links to gets the content,
// and the link is kept. A file that does not exist is created with the directories that it
// needs, at 0644 less the umask of the process.
func (r *Root) Write(p source.Path, content []byte) error {
	target, err := r.resolved(string(p))
	if err != nil {
		return err
	}
	mode, existed, err := r.prepare(target)
	if err != nil {
		return err
	}

	staged := target + Partial
	if err := r.root.WriteFile(staged, content, mode); err != nil {
		return fmt.Errorf("files: stage %s: %w", p, err)
	}
	if existed {
		// The umask of the process applied to the staged file, and the file keeps its mode.
		if err := r.root.Chmod(staged, mode); err != nil {
			return errors.Join(fmt.Errorf("files: set the mode of %s: %w", p, err), r.root.Remove(staged))
		}
	}
	if err := r.root.Rename(staged, target); err != nil {
		return errors.Join(fmt.Errorf("files: write %s: %w", p, err), r.root.Remove(staged))
	}
	return nil
}

// Move renames the file at from to to, with its mode, and creates the directories of to. It
// returns an error when a file or a link is at to.
func (r *Root) Move(from, to source.Path) error {
	if _, err := r.root.Lstat(string(to)); err == nil {
		return fmt.Errorf("files: move %s: a file is at %s", from, to)
	}
	if dir := path.Dir(string(to)); dir != "." {
		if err := r.root.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("files: make %s: %w", dir, err)
		}
	}
	if err := r.root.Rename(string(from), string(to)); err != nil {
		return fmt.Errorf("files: move %s: %w", from, err)
	}
	return nil
}

// Remove deletes the file at p. A path without a file is not an error, because the
// workspace is then as the caller asked.
func (r *Root) Remove(p source.Path) error {
	if err := r.root.Remove(string(p)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("files: remove %s: %w", p, err)
	}
	return nil
}

// resolved returns the path that a write to p changes: p, or the file that p links to when
// p is a symbolic link, through at most [links] symbolic links. An absolute link target is
// mapped into the directory. A relative target out of the directory fails when it is
// written, because [os.Root] refuses the path.
func (r *Root) resolved(p string) (string, error) {
	for range links {
		info, err := r.root.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return p, nil
		case err != nil:
			return "", fmt.Errorf("files: stat %s: %w", p, err)
		case info.Mode()&fs.ModeSymlink == 0:
			return p, nil
		}
		target, err := r.root.Readlink(p)
		if err != nil {
			return "", fmt.Errorf("files: read the link %s: %w", p, err)
		}
		if !filepath.IsAbs(target) {
			p = path.Join(path.Dir(p), filepath.ToSlash(target))
			continue
		}
		inside, err := filepath.Rel(r.dir, target)
		if err != nil || !filepath.IsLocal(inside) {
			return "", fmt.Errorf("files: %s links to %s, which is outside the workspace", p, target)
		}
		p = filepath.ToSlash(inside)
	}
	return "", fmt.Errorf("files: %s is behind more than %d links", p, links)
}

// prepare returns the mode of the file at p and reports whether the file exists. It creates
// the directories of a new file, whose mode is 0644.
func (r *Root) prepare(p string) (fs.FileMode, bool, error) {
	switch info, err := r.root.Stat(p); {
	case err == nil:
		return info.Mode().Perm(), true, nil
	case !errors.Is(err, fs.ErrNotExist):
		return 0, false, fmt.Errorf("files: stat %s: %w", p, err)
	}
	if dir := path.Dir(p); dir != "." {
		if err := r.root.MkdirAll(dir, 0o755); err != nil {
			return 0, false, fmt.Errorf("files: make %s: %w", dir, err)
		}
	}
	return 0o644, false, nil
}

const (
	// Partial is the suffix of a staged file. No language claims it, and it names techne.
	Partial = ".techne-partial"
	// links is the number of symbolic links that a write follows at most, as the kernel of
	// Linux does.
	links = 40
)
