// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	golang "go.dokimi.dev/techne/lang/go"
)

// layered returns a module in which b imports a, c imports b, and d imports nothing and does
// not compile. b calls F of a, and c calls the method M of the type T of a through the alias T
// of b.
func layered() map[string]string {
	return map[string]string{
		"a/a.go": "package a\n\ntype T struct{}\n\nfunc (T) M() int { return 1 }\n\nfunc F() int { return 1 }\n",
		"b/b.go": "package b\n\nimport \"example.com/p/a\"\n\ntype T = a.T\n\nfunc G() int { return a.F() }\n",
		"c/c.go": "package c\n\nimport \"example.com/p/b\"\n\nfunc H() int { return b.T{}.M() }\n",
		"d/d.go": "package d\n\nvar D int = \"d\"\n",
	}
}

// faulty returns the files of the errors of found, each once and sorted.
func faulty(found []edit.Finding) []source.Path {
	var out []source.Path
	for _, one := range found {
		if !slices.Contains(out, one.Diagnostic.Span.Path) {
			out = append(out, one.Diagnostic.Span.Path)
		}
	}
	slices.Sort(out)
	return out
}

func TestGraph(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			// old and now are the text of a/a.go that the change replaces and the text that it
			// writes.
			old, now string
			want     []source.Path
		}{
			{
				name: "returns no error of a package that imports no changed package",
				old:  "func F()", now: "func F()",
				want: nil,
			},
			{
				name: "returns the error of a package that imports a changed package",
				old:  "func F()", now: "func F2()",
				want: []source.Path{"b/b.go"},
			},
			{
				name: "returns the error of a package that imports a changed package through another",
				old:  "func (T) M()", now: "func (T) N()",
				want: []source.Path{"c/c.go"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				files := layered()
				changed := strings.Replace(files["a/a.go"], tt.old, tt.now, 1) + "\nfunc Extra() {}\n"
				got, err := serving(t, files).Check(t.Context(), map[source.Path][]byte{"a/a.go": []byte(changed)})
				assert.NoError(t, err, "Check of a change to a/a.go")
				assert.Equal(t, faulty(got.Items), tt.want, "the files with errors")
			})
		}

		t.Run("returns the error of a package that a file written after an earlier check makes an importer",
			func(t *testing.T) {
				t.Parallel()
				files := layered()
				root := workspace(t, files)
				e := over(t, root)
				changed := map[source.Path][]byte{
					"a/a.go": []byte(strings.Replace(files["a/a.go"], "func F()", "func F2()", 1)),
				}
				_, err := e.Check(t.Context(), changed)
				assert.NoError(t, err, "the first Check of a change to a/a.go")
				importer := "package d\n\nimport \"example.com/p/a\"\n\nvar E = a.F()\n"
				assert.NoError(t, os.WriteFile(filepath.Join(root, "d", "e.go"), []byte(importer), 0o600), "d/e.go")
				got, err := e.Check(t.Context(), changed)
				assert.NoError(t, err, "the second Check of the change to a/a.go")
				assert.Equal(t, faulty(got.Items), []source.Path{"b/b.go", "d/d.go", "d/e.go"}, "the files with errors")
			})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the error of the file of the scope in a module of several packages", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, layered()).Verify(t.Context(), engine.Request{Scope: "d/d.go"}, nil)
			assert.NoError(t, err, "Verify of d/d.go")
			assert.Equal(t, faulty(got.Items), []source.Path{"d/d.go"}, "the files with errors")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the uses in every package after a verify of one package", func(t *testing.T) {
			t.Parallel()
			e := serving(t, layered())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a"}, nil)
			assert.NoError(t, err, "Verify of a")
			got, err := e.Relate(t.Context(), engine.Request{Scope: "."},
				sema.NewID(golang.Language, "a", "F", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F")
			assert.Equal(t, edges(got.Items), []string{"G"}, "the declarations that use F")
		})
	})
}

// TestGraphEnv sets the environment of the process, so none of its cases runs in parallel.
func TestGraphEnv(t *testing.T) {
	t.Run("Check", func(t *testing.T) {
		t.Run("writes the files of the go command under the cache directory of the user", func(t *testing.T) {
			files := layered()
			e := over(t, workspace(t, files))
			cache := t.TempDir()
			// The build cache stays where it is, so the go command does not build the standard
			// library again under the new cache directory.
			builds := os.Getenv("GOCACHE")
			if builds == "" {
				user, err := os.UserCacheDir()
				assert.NoError(t, err, "the cache directory of the user")
				builds = filepath.Join(user, "go-build")
			}
			t.Setenv("GOCACHE", builds)
			t.Setenv("XDG_CACHE_HOME", cache)
			t.Setenv("GOTMPDIR", "")
			// -work keeps the work directory of the go command after it exits.
			t.Setenv("GOFLAGS", "-work")

			changed := strings.Replace(files["a/a.go"], "func F()", "func F2()", 1)
			got, err := e.Check(t.Context(), map[source.Path][]byte{"a/a.go": []byte(changed)})
			assert.NoError(t, err, "Check of a change to a/a.go")
			assert.Equal(t, faulty(got.Items), []source.Path{"b/b.go"}, "the files with errors")
			kept, err := filepath.Glob(filepath.Join(cache, "techne", "go", "go-build*"))
			assert.NoError(t, err, "the work directories under the cache directory")
			assert.NotEmpty(t, kept, "the work directories under the cache directory")
		})
	})
}
