// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"io/fs"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// ProjectOf returns the project that contains p: the nearest directory, at
// or above p, that contains a file matching one of manifests. It returns
// engine.Root when no directory does, when manifests is empty, and for a
// path outside the workspace.
//
// A manifest is a file name, such as "go.mod", or a path.Match pattern,
// such as "*.csproj". Compilation errors are a fact about one project, so
// engines lower the fidelity of an answer only when the project of its
// scope has errors.
func ProjectOf(fsys fs.FS, p source.Path, manifests []string) source.Path {
	at := path.Clean(string(p))
	if len(manifests) == 0 || path.IsAbs(at) {
		return engine.Root
	}
	for ; at != "." && at != "/"; at = path.Dir(at) {
		if marked(fsys, at, manifests) {
			return source.Path(at)
		}
	}
	return engine.Root
}

// Within reports whether p is inside project or is project itself. It
// compares whole path segments, so techne-lang-go is not inside
// techne-lang. Every workspace path is within engine.Root. An absolute
// path names a file outside the workspace, which is within no project.
func Within(p, project source.Path) bool {
	at := path.Clean(string(p))
	switch {
	case path.IsAbs(at):
		return false
	case project == engine.Root:
		return true
	}
	root := path.Clean(string(project))
	return at == root || strings.HasPrefix(at, root+"/")
}

// marked reports whether dir is a directory that contains a file matching
// one of manifests. It lists dir only for a manifest that is a pattern.
func marked(fsys fs.FS, dir string, manifests []string) bool {
	var entries []fs.DirEntry
	listed := false
	for _, manifest := range manifests {
		if !strings.ContainsAny(manifest, `*?[\`) {
			if info, err := fs.Stat(fsys, path.Join(dir, manifest)); err == nil && !info.IsDir() {
				return true
			}
			continue
		}
		if !listed {
			entries, _ = fs.ReadDir(fsys, dir)
			listed = true
		}
		for _, entry := range entries {
			if ok, err := path.Match(manifest, entry.Name()); err == nil && ok && !entry.IsDir() {
				return true
			}
		}
	}
	return false
}
