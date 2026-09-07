// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"io/fs"
	"path"

	"go.dokimi.dev/techne/core/source"
)

// ProjectOf is the directory a path compiles as part of: the nearest
// ancestor holding one of this language's manifests.
//
// # Why compilation is bounded here
//
// A workspace is not a compilation unit and a repository holding
// seventeen modules does not build or fail as one. Whether the code a
// question is about type-checks is a fact about its project, and reading
// it as a fact about the workspace makes every project answer for every
// other.
//
// Measured on one such repository: gopls reports "golang.org/x/mod is
// not in your go.mod file" against the root module at error severity.
// Read workspace-wide, that one untidy manifest withdrew the tier every
// write operation needs, in every module, for the life of the session.
//
// The workspace root when no manifest is found above the path, so a
// language that declares none, or a file outside any project, is
// answered about as a whole. [Declaration.Manifests] is what each
// language names its own by.
func ProjectOf(fsys fs.FS, p source.Path, manifests []string) source.Path {
	held := path.Clean(string(p))
	if held == "" || held == "." || path.IsAbs(held) {
		return Root
	}
	if len(manifests) == 0 {
		return Root
	}

	// From the path upwards, so the nearest manifest wins: a module
	// inside a workspace is its own project, and the workspace's own
	// manifest speaks only for what no nearer one claims.
	for at := held; at != "." && at != "/"; at = path.Dir(at) {
		if !holds(fsys, at) {
			continue
		}
		for _, name := range manifests {
			if _, err := fs.Stat(fsys, path.Join(at, name)); err == nil {
				return source.Path(at)
			}
		}
	}
	return Root
}

// Root is the workspace itself, and the project of anything no manifest
// claims.
const Root source.Path = "."

// holds reports whether a path names a directory, so a file's own name
// is not searched for a manifest inside it.
func holds(fsys fs.FS, at string) bool {
	info, err := fs.Stat(fsys, at)
	return err == nil && info.IsDir()
}

// Within reports whether a path is inside a project.
//
// Prefix matching on whole segments, so techne-lang-go is not read as
// being inside techne-lang.
func Within(p, project source.Path) bool {
	if project == Root {
		return true
	}
	held, root := path.Clean(string(p)), path.Clean(string(project))
	return held == root || len(held) > len(root) &&
		held[:len(root)] == root && held[len(root)] == '/'
}
