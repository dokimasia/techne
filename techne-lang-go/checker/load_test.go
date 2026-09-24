// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
)

// internal is a test file inside package p that calls Target on line 5.
const internal = "package p\n\nimport \"testing\"\n\n" +
	"func TestTarget(t *testing.T) {\n\tif Target() != 1 {\n\t\tt.Fatal()\n\t}\n}\n"

// tested returns the files of a package with a caller of Target and a test file inside the
// package.
func tested() map[string]string {
	return map[string]string{
		"lib.go":               "package p\n\nfunc Target() int { return 1 }\n",
		"use.go":               "package p\n\nfunc Use() int { return Target() }\n",
		"lib_internal_test.go": internal,
	}
}

// boxed is the file of module a, which declares Box and Target. uses is the file of module b,
// which calls Target and reads Box.Size on line 4.
const (
	boxed = "package x\n\ntype Box struct {\n\tSize int\n}\n\nfunc Target() int { return 1 }\n"
	uses  = "package y\n\nimport \"example.com/a/x\"\n\nfunc Use() int { return x.Target() + x.Box{}.Size }\n"
)

// alone is a go.mod file of module b without requirements, and replaced is one that replaces
// module a with its directory.
const (
	alone    = "module example.com/b\n\ngo 1.24\n"
	replaced = "module example.com/b\n\ngo 1.24\n\nrequire example.com/a v0.0.0\n\nreplace example.com/a => ../a\n"
)

// two returns the files of modules a and b, with manifest as the go.mod file of b.
func two(manifest string) map[string]string {
	return map[string]string{
		"a/go.mod": "module example.com/a\n\ngo 1.24\n",
		"a/x/x.go": boxed,
		"b/go.mod": manifest,
		"b/y/y.go": uses,
	}
}

// target is the ID of Target of module a of [two].
var target = sema.NewID(golang.Language, "a/x", "Target", sema.KindFunction)

// ranged loops over an integer, which Go 1.22 and later compile.
const ranged = "package p\n\nfunc Loop() (n int) {\n\tfor i := range 3 {\n\t\tn += i\n\t}\n\treturn n\n}\n"

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the errors of a file that changed after the last question", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, whole())
			e := over(t, root)
			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify before the change")
			assert.Empty(t, got.Items, "the errors before the change")

			assert.NoError(t, os.WriteFile(filepath.Join(root, "use.go"), []byte(broken), 0o644), "WriteFile use.go")
			got, err = e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify after the change")
			assert.NotEmpty(t, got.Items, "the errors after the change")
		})

		t.Run("returns the errors of a file added after the last question", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, whole())
			e := over(t, root)
			_, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify before the file")

			assert.NoError(t, os.WriteFile(filepath.Join(root, "extra.go"), []byte(broken), 0o644),
				"WriteFile extra.go")
			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify after the file")
			assert.NotEmpty(t, got.Items, "the errors of extra.go")
		})

		t.Run("returns no error after an edit to go.mod repairs the build", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{"go.mod": "module example.com/p\n\ngo 1.21\n", "loop.go": ranged})
			e := over(t, root)
			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify under go 1.21")
			assert.NotEmpty(t, got.Items, "the errors of a range over an integer under go 1.21")

			manifest := []byte("module example.com/p\n\ngo 1.24.0\n")
			assert.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), manifest, 0o644), "WriteFile go.mod")
			got, err = e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify under go 1.24")
			assert.Empty(t, got.Items, "the errors under go 1.24")
		})

		t.Run("returns the errors of every module of a workspace without a go.work file", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{
				"a/go.mod": "module example.com/a\n\ngo 1.24\n",
				"a/a.go":   "package a\n\nvar A int = \"a\"\n",
				"b/go.mod": "module example.com/b\n\ngo 1.24\n",
				"b/b.go":   "package b\n\nvar B int = \"b\"\n",
			})
			got, err := over(t, root).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of two modules")
			var paths []string
			for _, one := range got.Items {
				paths = append(paths, string(one.Diagnostic.Span.Path))
			}
			assert.Equal(t, paths, []string{"a/a.go", "b/b.go"}, "the files of the errors")
		})

		t.Run("returns the errors of a module that the go.work file leaves out", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{
				"go.work":  "go 1.24\n\nuse ./a\n",
				"a/go.mod": "module example.com/a\n\ngo 1.24\n",
				"a/a.go":   "package a\n",
				"c/go.mod": "module example.com/c\n\ngo 1.24\n",
				"c/c.go":   "package c\n\nvar C int = \"c\"\n",
			})
			got, err := over(t, root).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a module outside the go.work file")
			assert.Length(t, got.Items, 1, "the errors of module c")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("c/c.go"), "the file of the error")
		})

		t.Run("returns a partial answer that names a module that fails to load", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{
				"a/go.mod": "module example.com/a\n\ngo 1.24\n",
				"a/a.go":   "package a\n\nvar A int = \"a\"\n",
				"b/go.mod": "module example.com/b\n\ngo 1.24\n\nrequire (\n",
				"b/b.go":   "package b\n",
			})
			got, err := over(t, root).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify with a module that fails to load")
			assert.Length(t, got.Items, 1, "the errors of module a")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, carries(got.Caveats, trust.CaveatUnsupported), "the caveat of the module that fails")
			assert.Contains(t, got.Caveats[0].Paths, source.Path("b"), "the module that fails to load")
		})

		t.Run("returns the errors of a root below the directory of its go.mod file", func(t *testing.T) {
			t.Parallel()
			module := written(t, map[string]string{
				"go.mod":     "module example.com/m\n\ngo 1.24\n",
				"sub/sub.go": "package sub\n\nvar S int = \"s\"\n",
			})
			got, err := over(t, filepath.Join(module, "sub")).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a root inside a module")
			assert.Length(t, got.Items, 1, "the errors under the root")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("sub.go"), "the file of the error")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a caller in an in-package test file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, tested()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Target", sema.KindFunction), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Target")
			assert.Equal(t, places(got.Items), []string{"lib_internal_test.go:5", "use.go:2"}, "the sites of the calls")
		})

		t.Run("returns a caller in a package that imports a package with tests", func(t *testing.T) {
			t.Parallel()
			files := importing()
			files["p/p_internal_test.go"] = "package p\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) {}\n"
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "."},
				sema.NewID(golang.Language, "p", "Target", sema.KindFunction), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Target")
			assert.Equal(t, places(got.Items), []string{"q/q.go:4"}, "the site of the call in package q")
		})

		t.Run("returns each use in a package with tests once", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, tested()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Target", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Target")
			assert.Equal(t, edges(got.Items), []string{"TestTarget", "Use"}, "the uses of Target")
		})

		t.Run("returns a caller in another module of a go.work file", func(t *testing.T) {
			t.Parallel()
			files := two(alone)
			files["go.work"] = "go 1.24\n\nuse (\n\t./a\n\t./b\n)\n"
			got, err := over(t, written(t, files)).Relate(t.Context(), engine.Request{Scope: "."},
				target, sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Target")
			assert.Equal(t, places(got.Items), []string{"b/y/y.go:4"}, "the site of the call in module b")
		})

		t.Run("returns a caller in another module that replaces the module of the declaration", func(t *testing.T) {
			t.Parallel()
			got, err := over(t, written(t, two(replaced))).Relate(t.Context(), engine.Request{Scope: "."},
				target, sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Target")
			assert.Equal(t, places(got.Items), []string{"b/y/y.go:4"}, "the site of the call in module b")
		})

		t.Run("returns workspace paths for a root under a symbolic link", func(t *testing.T) {
			t.Parallel()
			link := filepath.Join(t.TempDir(), "link")
			assert.NoError(t, os.Symlink(workspace(t, whole()), link), "Symlink")
			got, err := over(t, link).Relate(t.Context(), engine.Request{Scope: "."},
				subject("helper", sema.KindFunction), sema.CalledBy)
			assert.NoError(t, err, "Relate under a symbolic link")
			assert.Equal(t, places(got.Items), []string{"store.go:9"}, "the site of the call of helper")
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a list error of a module at the path of the module", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{
				"a/go.mod": "module example.com/a\n\ngo 1.24\n",
				"a/a.go":   "package a\n",
			})
			got, err := over(t, root).Check(t.Context(), map[source.Path][]byte{"a/empty.go": {}})
			assert.NoError(t, err, "Check of an empty file")
			assert.NotEmpty(t, got.Items, "the errors of the empty file")
			for _, one := range got.Items {
				assert.Equal(t, one.Diagnostic.Span.Path, source.Path("a/empty.go"),
					"the file of "+one.Diagnostic.Message)
			}
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration of a use in an in-package test file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, tested()).Resolve(t.Context(), engine.Request{Scope: "lib_internal_test.go"},
				at(t, internal, "if Target"))
			assert.NoError(t, err, "Resolve in the test file")
			assert.Equal(t, names(got.Items), []string{"Target"}, "the declaration of the use")
			assert.Equal(t, got.Items[0].Span.Path, source.Path("lib.go"), "the file of the declaration")
		})

		t.Run("returns a declaration of the standard library with its line", func(t *testing.T) {
			t.Parallel()
			printed := "package p\n\nimport \"fmt\"\n\nfunc Print() { fmt.Println() }\n"
			got, err := serving(t, map[string]string{"print.go": printed}).Resolve(t.Context(),
				engine.Request{Scope: "print.go"}, at(t, printed, "fmt.Println"))
			assert.NoError(t, err, "Resolve of fmt.Println")
			assert.Length(t, got.Items, 1, "the declarations of fmt.Println")
			found := got.Items[0]
			assert.HasSuffix(t, string(found.Span.Path), "/fmt/print.go", "the file of Println")
			content, err := os.ReadFile(filepath.FromSlash(string(found.Span.Path)))
			assert.NoError(t, err, "ReadFile print.go")
			assert.True(t, strings.HasPrefix(string(content[found.Span.Start.Offset:]), "Println("),
				"the offset of Println in print.go")
			assert.Equal(t, found.Signature, "func Println(a ...any) (n int, err error)", "the signature of Println")
		})

		t.Run("returns a declaration of another module with its span from source", func(t *testing.T) {
			t.Parallel()
			got, err := over(t, written(t, two(replaced))).Resolve(t.Context(), engine.Request{Scope: "b/y/y.go"},
				at(t, uses, "}.Size"))
			assert.NoError(t, err, "Resolve of Box.Size")
			assert.Length(t, got.Items, 1, "the declarations of Size")
			assert.Equal(t, got.Items[0].ID, sema.NewID(golang.Language, "a/x", "Box.Size", sema.KindField),
				"the ID of Size")
			assert.Equal(t, got.Items[0].Span, source.Span{
				Path:  "a/x/x.go",
				Start: source.Position{Offset: 30, Line: 3, Column: 1},
				End:   source.Position{Offset: 34, Line: 3, Column: 5},
			}, "the span of Size")
		})
	})
}
