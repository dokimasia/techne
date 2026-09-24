// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"fmt"
	"io/fs"
	"path"
	"slices"

	"go.dokimi.dev/techne/core/source"
)

// Files are the files a walk returns.
type Files struct {
	// Read are the claimed files of at most Largest bytes, sorted.
	Read []source.Path
	// Unread are the claimed files larger than Largest, sorted. An engine
	// reports them in a trust.CaveatUnread.
	Unread []source.Path
}

// Walk returns the files in scope whose extension is one of extensions.
// The scope is a file or a directory, relative to the root of fsys.
//
// Walk does not enter a directory that [Vendored] names or that the
// .gitignore files of the workspace exclude, unless that directory is the
// scope. It returns [GeneratedError] when the .gitignore files exclude the
// scope, and an error when the scope does not exist. A file scope with an
// extension outside extensions returns no files.
func Walk(fsys fs.FS, scope source.Path, extensions []string) (Files, error) {
	name, info, rules, err := located(fsys, scope)
	if err != nil {
		return Files{}, err
	}

	var out Files
	if !info.IsDir() {
		if Claims(name, extensions) {
			out.add(source.Path(name), info.Size())
		}
		return out, nil
	}

	err = fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if p != name && (Vendored(path.Base(p)) || rules.skips(p, true)) {
				return fs.SkipDir
			}
			rules.read(fsys, p)
			return nil
		case !Claims(p, extensions) || rules.skips(p, false):
			return nil
		}
		if size, ok := sizeOf(fsys, p, d); ok {
			out.add(source.Path(p), size)
		}
		return nil
	})
	if err != nil {
		return Files{}, fmt.Errorf("lang: walk %q: %w", scope, err)
	}
	slices.Sort(out.Read)
	slices.Sort(out.Unread)
	return out, nil
}

// Claims reports whether the extension of p is one of extensions. It reads
// no filesystem, so it also applies to a path that does not exist.
func Claims(p string, extensions []string) bool {
	suffix := path.Ext(p)
	return suffix != "" && slices.Contains(extensions, suffix)
}

// add records p in Read or Unread by its size.
func (f *Files) add(p source.Path, size int64) {
	if size > Largest {
		f.Unread = append(f.Unread, p)
		return
	}
	f.Read = append(f.Read, p)
}

// located cleans scope, reads the .gitignore files of every directory above
// it, and returns the result with the FileInfo of scope. It returns
// GeneratedError when those files exclude scope, and an error when scope
// does not exist. Walk and Readable call it before they apply their own
// rules.
func located(fsys fs.FS, scope source.Path) (string, fs.FileInfo, *ignores, error) {
	name := path.Clean(string(scope))
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return "", nil, nil, fmt.Errorf("lang: %q: %w", scope, err)
	}
	rules := &ignores{}
	for _, dir := range ancestors(name) {
		rules.read(fsys, dir)
	}
	if rules.excludes(name, info.IsDir()) {
		return "", nil, nil, GeneratedError{Scope: scope}
	}
	return name, info, rules, nil
}

// sizeOf returns the size of the file at p, following a symbolic link. It
// reports false for:
//
//   - a link to a directory
//   - a link to nothing
//   - a file removed during the walk
func sizeOf(fsys fs.FS, p string, d fs.DirEntry) (int64, bool) {
	info, err := d.Info()
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		info, err = fs.Stat(fsys, p)
	}
	if err != nil || info.IsDir() {
		return 0, false
	}
	return info.Size(), true
}
