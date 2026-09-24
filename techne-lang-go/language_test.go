// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"os"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language go", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Declaration().Language, golang.Language, "the language of the declaration")
			assert.Equal(t, string(golang.Language), "go", "the value of Language")
		})

		t.Run("claims the extension of Go", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Declaration().Extensions, []string{".go"}, "the extensions of Go")
		})

		t.Run("lists the manifests of a Go project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Declaration().Manifests, []string{"go.mod", "go.work"}, "the manifests of Go")
		})

		t.Run("returns the directory as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Declaration().Namespace("core/trust/status.go"), "core/trust",
				"the unit of core/trust/status.go")
		})

		t.Run("ignores the blank identifier", func(t *testing.T) {
			t.Parallel()
			assert.True(t, golang.Declaration().Blank["_"], "the blank identifiers of Go")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs gopls serve", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().Command, []string{"gopls", "serve"}, "the command of gopls")
		})

		t.Run("opens a file as go", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().LanguageID, lsp.IdentityGo, "the language identifier of gopls")
		})

		t.Run("extracts a function by the kind of its code action", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().Extracts, lsp.Refactor{Kind: "refactor.extract.function"},
				"the extraction of gopls")
		})
	})

	t.Run("Grammar", func(t *testing.T) {
		t.Parallel()

		t.Run("qualifies a method by the type of its receiver", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"store.go": {Data: []byte("package store\n\n" +
				"type Store struct{}\n\nfunc (s *Store) Get() int { return 0 }\n\n" +
				"type Cache[T any] struct{}\n\nfunc (c *Cache[T]) Get() int { return 0 }\n")}}
			e, err := treesitter.New(fsys, golang.Declaration(), golang.Grammar())
			assert.NoError(t, err, "New of the Go engine")
			t.Cleanup(e.Close)
			got, err := e.Outline(t.Context(), engine.Request{Scope: "store.go"})
			assert.NoError(t, err, "Outline of store.go")
			var methods []string
			for _, one := range got.Items {
				if one.Kind == sema.KindMethod {
					methods = append(methods, one.ID.Name())
				}
			}
			assert.Equal(t, methods, []string{"Store.Get", "Cache.Get"}, "the qualified names of the methods")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("adds the type checker for a workspace on disk", func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			c := engine.NewCatalog()
			assert.NoError(t, golang.Register(lang.Workspace{FS: os.DirFS(root), Root: root}, lang.NewRegistry(), c),
				"Register of a workspace on disk")
			assert.Contains(t, engines(t, c), "go/types", "the engines of a workspace on disk")
		})

		t.Run("leaves out the type checker for a workspace in memory", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			assert.NoError(t, golang.Register(lang.Workspace{FS: fstest.MapFS{}}, lang.NewRegistry(), c),
				"Register of a workspace in memory")
			assert.NotContains(t, engines(t, c), "go/types", "the engines of a workspace in memory")
		})

		t.Run("returns an error for a root that is not a directory", func(t *testing.T) {
			t.Parallel()
			file := t.TempDir() + "/file"
			assert.NoError(t, os.WriteFile(file, nil, 0o600), "WriteFile of "+file)
			err := golang.Register(lang.Workspace{FS: fstest.MapFS{}, Root: file},
				lang.NewRegistry(), engine.NewCatalog())
			assert.HasError(t, err, "Register of a root that is a file")
		})
	})
}

// engines returns the distinct names of the engines in c.
func engines(t *testing.T, c *engine.Catalog) []string {
	t.Helper()
	var out []string
	for _, one := range c.Capabilities(t.Context()) {
		if !slices.Contains(out, one.Engine) {
			out = append(out, one.Engine)
		}
	}
	return out
}
