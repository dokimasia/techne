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
		if !claims(name, extensions) {
			return nil, nil
		}
		return []source.Path{source.Path(name)}, nil
	}

	var out []source.Path
	walkErr := fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir(), !claims(p, extensions):
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

// claims reports whether one of the extensions covers a path.
func claims(p string, extensions []string) bool {
	suffix := path.Ext(p)
	return suffix != "" && slices.Contains(extensions, suffix)
}
