// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// A repository is not a compilation unit. Seventeen modules in one
// workspace do not build or fail together, and whether the code a
// question is about type-checks is a fact about its project.
func TestProjectOf(t *testing.T) {
	t.Parallel()

	held := fstest.MapFS{
		"go.mod":                    {Data: []byte("module root\n")},
		"main.go":                   {Data: []byte("package main")},
		"lang/go.mod":               {Data: []byte("module lang\n")},
		"lang/read.go":              {Data: []byte("package lang")},
		"lang/lsp/engine.go":        {Data: []byte("package lsp")},
		"lang-go/go.mod":            {Data: []byte("module langgo\n")},
		"lang-go/checker/load.go":   {Data: []byte("package checker")},
		"unclaimed/notes/note.go":   {Data: []byte("package notes")},
		"unclaimed/notes/README.md": {Data: []byte("")},
	}
	manifests := []string{"go.mod", "go.work"}

	t.Run("is the nearest manifest above a path", func(t *testing.T) {
		t.Parallel()
		// The nearest, so a module inside a workspace is its own project
		// and the workspace's manifest speaks only for what no nearer
		// one claims.
		for path, want := range map[source.Path]source.Path{
			"lang/read.go":            "lang",
			"lang/lsp/engine.go":      "lang",
			"lang-go/checker/load.go": "lang-go",
			"main.go":                 ".",
			"unclaimed/notes/note.go": ".",
		} {
			assert.Equal(t, lang.ProjectOf(held, path, manifests), want,
				"the project of "+string(path))
		}
	})

	t.Run("is the workspace where the language names no manifest", func(t *testing.T) {
		t.Parallel()
		// A language that declares none has no project boundary to
		// speak of, and is answered about as a whole rather than
		// refused.
		assert.Equal(t, lang.ProjectOf(held, "lang/read.go", nil), lang.Root,
			"no manifest to look for is the workspace")
	})

	t.Run("is the workspace for a path outside it", func(t *testing.T) {
		t.Parallel()
		// A server answers about a standard library and a module cache
		// as well as about the workspace, and those keep their absolute
		// form. Nothing here speaks for them.
		assert.Equal(t, lang.ProjectOf(held, "/usr/lib/go/src/fmt/print.go", manifests),
			lang.Root, "a path outside the workspace belongs to no project in it")
	})
}

// A diagnostic is about the project it was reported in, and a project is
// told from its siblings by whole path segments rather than by prefix.
func TestWithin(t *testing.T) {
	t.Parallel()

	t.Run("holds a path under the project", func(t *testing.T) {
		t.Parallel()
		assert.True(t, lang.Within("lang/lsp/engine.go", "lang"), "under it")
		assert.True(t, lang.Within("lang", "lang"), "and the project itself")
	})

	t.Run("does not hold a sibling whose name starts the same", func(t *testing.T) {
		t.Parallel()
		// techne-lang-go is not inside techne-lang, and reading it as
		// though it were is what makes one module answer for another.
		assert.False(t, lang.Within("lang-go/checker/load.go", "lang"),
			"a sibling sharing a prefix is a different project")
	})

	t.Run("holds everything when the project is the workspace", func(t *testing.T) {
		t.Parallel()
		assert.True(t, lang.Within("anywhere/at/all.go", lang.Root),
			"a workspace-wide claim covers the workspace")
	})
}
