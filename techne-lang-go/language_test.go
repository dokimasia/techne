// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	assertfiles "go.dokimi.dev/assert/files"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
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

		t.Run("removes a file with a build constraint that no build satisfies", func(t *testing.T) {
			t.Parallel()
			removed := golang.Declaration().Removed
			header, _, _ := strings.Cut(string(removed), "\n")
			expr, err := constraint.Parse(header)
			assert.NoError(t, err, "the build constraint of the removed content")
			for _, every := range []bool{false, true} {
				assert.False(t, expr.Eval(func(string) bool { return every }), "the constraint under a build")
			}
			_, err = parser.ParseFile(token.NewFileSet(), "removed.go", removed, parser.PackageClauseOnly)
			assert.NoError(t, err, "the parse of the removed content")
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

		t.Run("leaves the importers of a package to the type checker", func(t *testing.T) {
			t.Parallel()
			assert.False(t, golang.Server().Imports, "Imports of gopls")
		})

		t.Run("runs gopls serve", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().Command, []string{"gopls", "serve"}, "the command of gopls")
		})

		t.Run("leaves embedding to the type checker", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().Unrelated, []sema.RelationKind{sema.Embeds, sema.EmbeddedBy},
				"the relations that gopls answers with other relations")
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

		t.Run("asks gopls to report each diagnosis as a progress job", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().Settings["verboseWorkDoneProgress"], any(true),
				"the setting verboseWorkDoneProgress of gopls")
		})

		t.Run("names a diagnosis of gopls by the start of its title", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().Diagnosis, "diagnosing", "the diagnosis of gopls")
		})

		t.Run("verifies a package against an imported package that changed on disk", func(t *testing.T) {
			t.Parallel()
			if _, err := exec.LookPath(golang.Server().Command[0]); err != nil {
				t.Skip("gopls is not on PATH, so no verify of gopls runs")
			}
			root := lsptest.Workspace(t, map[string]string{
				"go.mod": "module example.test/stale\n\ngo 1.27\n",
				"a/a.go": "package a\n\n// T is a value with one field.\ntype T struct{ X int }\n",
				"b/b.go": "package b\n\nimport \"example.test/stale/a\"\n\n// V is a value of T.\nvar V = a.T{X: 1}\n",
			})
			served, err := lsp.New(root, golang.Declaration(), golang.Server(), nil)
			assert.NoError(t, err, "lsp.New over "+root)
			lsptest.Cleanup(t, served)
			got, err := served.Verify(t.Context(), engine.Request{Scope: "b"}, nil)
			assert.NoError(t, err, "Verify of b")
			assert.Empty(t, got.Items, "the findings of b before the change")

			changed := assertfiles.Tree{
				"a/a.go": assertfiles.Text("package a\n\n// T is a value with two fields.\n" +
					"type T struct{ X, Y int }\n"),
				"b/b.go": assertfiles.Text("package b\n\nimport \"example.test/stale/a\"\n\n// V is a value of T.\n" +
					"var V = a.T{X: 1, Y: 2}\n"),
			}
			assertfiles.Write(t, root, changed)
			got, err = served.Verify(t.Context(), engine.Request{Scope: "b"}, nil)
			assert.NoError(t, err, "Verify of b after a and b change on disk")
			assert.Empty(t, got.Items, "the findings of b after the change")
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

		t.Run("outlines the declarations after a method with type parameters", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"generic.go": {Data: []byte("package generic\n\n" +
				"type Box[T any] struct{ v T }\n\n" +
				"func (b Box[T]) Map[U any](f func(T) U) Box[U] { return Box[U]{f(b.v)} }\n\n" +
				"func After() {}\n")}}
			e, err := treesitter.New(fsys, golang.Declaration(), golang.Grammar())
			assert.NoError(t, err, "New of the Go engine")
			t.Cleanup(e.Close)
			got, err := e.Outline(t.Context(), engine.Request{Scope: "generic.go"})
			assert.NoError(t, err, "Outline of generic.go")
			var methods, functions []string
			for _, one := range got.Items {
				switch one.Kind {
				case sema.KindMethod:
					methods = append(methods, one.ID.Name())
				case sema.KindFunction:
					functions = append(functions, one.ID.Name())
				default:
					// The case compares the methods and the functions alone.
				}
			}
			expect.Equal(t, methods, []string{"Box.Map"}, "the methods of generic.go")
			expect.Equal(t, functions, []string{"After"}, "the functions of generic.go")
			expect.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the outline")
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

		t.Run("serves gopls through Constrained for a workspace on disk", func(t *testing.T) {
			t.Parallel()
			if _, err := exec.LookPath(golang.Server().Command[0]); err != nil {
				t.Skip("gopls is not on PATH, so the catalogue serves no engine of it")
			}
			root := t.TempDir()
			c := engine.NewCatalog()
			assert.NoError(t, golang.Register(lang.Workspace{FS: os.DirFS(root), Root: root}, lang.NewRegistry(), c),
				"Register of a workspace on disk")
			var served []engine.Engine
			for _, one := range c.For(t.Context(), golang.Language, engine.RoleRelate) {
				if one.Name() == golang.Server().Name {
					served = append(served, one)
				}
			}
			assert.Length(t, served, 1, "the engines of gopls")
			_, bare := served[0].(*lsp.Engine)
			assert.False(t, bare, "the engine of gopls is the engine of the server alone")
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
			assert.Contains(t, err.Error(), "checker: go workspace root "+file+" is not a directory",
				"the error of Register")
		})

		t.Run("returns the error of the parser for a workspace on disk without a filesystem", func(t *testing.T) {
			t.Parallel()
			err := golang.Register(lang.Workspace{Root: t.TempDir()}, lang.NewRegistry(), engine.NewCatalog())
			assert.HasError(t, err, "Register of a workspace without a filesystem")
			assert.Contains(t, err.Error(), "no filesystem to read from", "the error of Register")
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
