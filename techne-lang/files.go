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

// FilesIn returns the files in a scope that a language claims, in a
// stable order.
//
// The scope is one file or one directory. A directory is walked; a file
// yields itself. A file the language does not claim yields nothing
// rather than an error, because a directory holding several languages is
// the normal case and every engine walking one meets it.
//
// A scope that does not exist is an error. Answering nothing would read
// as "this directory declares nothing", which is a different fact from
// "there is no such directory".
//
// A directory holding code the workspace did not write is not walked;
// see [Vendored] for which and why. The scope itself is never skipped,
// so a caller that names one is answered about it.
//
// Every engine reading files needs this, so it lives here rather than in
// one of them.
func FilesIn(fsys fs.FS, scope source.Path, extensions []string) ([]source.Path, error) {
	name := path.Clean(string(scope))
	if name == "" {
		name = "."
	}

	info, err := fs.Stat(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("lang: scope %q: %w", scope, err)
	}

	if !info.IsDir() {
		if !Claims(name, extensions) {
			return nil, nil
		}
		return []source.Path{source.Path(name)}, nil
	}

	var out []source.Path
	walkErr := fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			// The scope itself is never skipped. A caller that named a
			// dependency directory asked about it, and answering nothing
			// would report it as empty.
			if p != name && Vendored(path.Base(p)) {
				return fs.SkipDir
			}
			return nil
		case !Claims(p, extensions):
			return nil
		}
		out = append(out, source.Path(p))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("lang: walk %q: %w", scope, walkErr)
	}
	slices.Sort(out)
	return out, nil
}

// Claims reports whether one of the extensions covers a path.
//
// The rule is the file's extension and nothing else, so it needs no
// filesystem and answers about a path that does not exist. An engine
// that must decide whether a scope is one of its files before it reads
// anything asks this rather than carrying its own copy: two copies of
// the rule are two answers to "which language owns this path", and a
// read and a write that disagree plan a change with one engine and gate
// it with another.
func Claims(p string, extensions []string) bool {
	suffix := path.Ext(p)
	return suffix != "" && slices.Contains(extensions, suffix)
}
