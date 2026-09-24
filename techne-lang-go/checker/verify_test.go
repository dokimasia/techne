// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// importing are the files of a package p with Target and a package q that calls it.
func importing() map[string]string {
	return map[string]string{
		"p/p.go": "package p\n\nfunc Target() int { return 1 }\n",
		"q/q.go": "package q\n\nimport \"example.com/p/p\"\n\nvar V = p.Target()\n",
	}
}

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the errors of the type checker", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["broken.go"] = broken
			got, err := serving(t, files).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a module with an error")
			assert.Length(t, got.Items, 1, "the errors of the module")
			assert.Equal(t, got.Items[0].Diagnostic.Severity, diag.SeverityError, "the severity of the error")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("broken.go"), "the file of the error")
		})

		t.Run("returns the offset and the source line of an error", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["broken.go"] = broken
			got, err := serving(t, files).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a module with an error")
			found := got.Items[0].Diagnostic
			line := strings.Index(broken, "\treturn held.missing")
			assert.Equal(t, found.Span.Start.Line, 4, "the line of the error")
			assert.Equal(t, found.Span.Start.Offset, line+found.Span.Start.Column, "the offset of the error")
			assert.Equal(t, found.Snippet, "return held.missing", "the source line of the error")
		})

		t.Run("returns the file of an error in a directory whose name contains a colon", func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "with:colon")
			assert.NoError(t, os.Mkdir(root, 0o755), "Mkdir with:colon")
			for name, body := range map[string]string{"go.mod": module, "store.go": store, "broken.go": broken} {
				assert.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o644), "WriteFile "+name)
			}
			got, err := over(t, root).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify under with:colon")
			assert.Length(t, got.Items, 1, "the errors under with:colon")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("broken.go"), "the file of the error")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Start.Line, 4, "the line of the error")
		})

		t.Run("returns no error for a module that compiles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a module that compiles")
			assert.Empty(t, got.Items, "the errors of the module")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns the errors of the scope of the request", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["broken.go"] = broken
			got, err := serving(t, files).Verify(t.Context(), engine.Request{Scope: "store.go"}, nil)
			assert.NoError(t, err, "Verify of store.go")
			assert.Empty(t, got.Items, "the errors of store.go")
		})

		t.Run("leaves out the errors of a test file without tests", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["bad_test.go"] = "package p\n\nvar T int = \"t\"\n"
			got, err := serving(t, files).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify without tests")
			assert.Empty(t, got.Items, "the errors without tests")
		})

		t.Run("returns the errors of a test file with tests", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["bad_test.go"] = "package p\n\nvar T int = \"t\"\n"
			got, err := serving(t, files).Verify(t.Context(), engine.Request{Scope: ".", Tests: true}, nil)
			assert.NoError(t, err, "Verify with tests")
			assert.Length(t, got.Items, 1, "the errors with tests")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("bad_test.go"), "the file of the error")
		})

		t.Run("returns a skipped result for a scope without a Go file", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["docs/notes.md"] = "# notes\n"
			got, err := serving(t, files).Verify(t.Context(), engine.Request{Scope: "docs"}, nil)
			assert.NoError(t, err, "Verify of a scope without Go files")
			assert.True(t, got.Skipped, "the answer is skipped")
		})

		t.Run("returns a caveat that names the suites that it does not run", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Verify(t.Context(), engine.Request{Scope: "."}, []string{"vet"})
			assert.NoError(t, err, "Verify with the suite vet")
			assert.Equal(t, got.Caveats, []trust.Caveat{{
				Code: trust.CaveatUnsupported,
				Note: "the type checker runs its own analysis and none of these suites: vet",
			}}, "the caveats of the answer")
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the errors of content that is not on disk", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Check(t.Context(), map[source.Path][]byte{"use.go": []byte(broken)})
			assert.NoError(t, err, "Check of broken content")
			assert.NotEmpty(t, got.Items, "the errors of the content")
		})

		t.Run("returns no error for content that compiles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Check(t.Context(), map[source.Path][]byte{"use.go": []byte(use + "\n")})
			assert.NoError(t, err, "Check of content that compiles")
			assert.Empty(t, got.Items, "the errors of the content")
		})

		t.Run("returns an error in a file of the package that the change leaves out", func(t *testing.T) {
			t.Parallel()
			renamed := "package p\n\ntype Vault struct{ size int }\n\nfunc helper() int { return 1 }\n"
			got, err := serving(t, whole()).Check(t.Context(), map[source.Path][]byte{"store.go": []byte(renamed)})
			assert.NoError(t, err, "Check of a rename of Store")
			assert.NotEmpty(t, got.Items, "the errors of the rename")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("use.go"), "the file of the first error")
		})

		t.Run("returns an error in a package that imports a changed package", func(t *testing.T) {
			t.Parallel()
			changed := "package p\n\nfunc Target(n int) int { return n }\n"
			got, err := serving(t, importing()).Check(t.Context(), map[source.Path][]byte{"p/p.go": []byte(changed)})
			assert.NoError(t, err, "Check of a new parameter of Target")
			assert.Length(t, got.Items, 1, "the errors of the change")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("q/q.go"), "the file of the error")
		})

		t.Run("returns no error for a deleted file that nothing uses", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["extra.go"] = "package p\n\nfunc Extra() int { return 1 }\n"
			got, err := serving(t, files).Check(t.Context(), map[source.Path][]byte{"extra.go": nil})
			assert.NoError(t, err, "Check of the deletion of extra.go")
			assert.Empty(t, got.Items, "the errors of the deletion")
		})

		t.Run("returns an error for a deleted file that another file uses", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Check(t.Context(), map[source.Path][]byte{"store.go": nil})
			assert.NoError(t, err, "Check of the deletion of store.go")
			assert.NotEmpty(t, got.Items, "the errors of the deletion")
			assert.Equal(t, got.Items[0].Diagnostic.Span.Path, source.Path("use.go"), "the file of the first error")
		})

		t.Run("returns the errors of the cached view for content equal to the disk", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, whole())
			e := over(t, root)
			_, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify before the rewrite")

			full := filepath.Join(root, "use.go")
			info, err := os.Stat(full)
			assert.NoError(t, err, "Stat use.go")
			stale := strings.Replace(use, "Total", "Tota1", 1)
			assert.NoError(t, os.WriteFile(full, []byte(stale), 0o644), "WriteFile use.go")
			assert.NoError(t, os.Chtimes(full, info.ModTime(), info.ModTime()), "Chtimes use.go")

			got, err := e.Check(t.Context(), map[source.Path][]byte{"use.go": []byte(stale)})
			assert.NoError(t, err, "Check of the content on disk")
			assert.Empty(t, got.Items, "the errors of the cached view")
			fresh, err := over(t, root).Check(t.Context(), map[source.Path][]byte{"use.go": []byte(stale)})
			assert.NoError(t, err, "Check of a new engine")
			assert.NotEmpty(t, fresh.Items, "the errors of the content on disk")
		})

		t.Run("keeps the view of the disk after a check", func(t *testing.T) {
			t.Parallel()
			e := serving(t, whole())
			_, err := e.Check(t.Context(), map[source.Path][]byte{"use.go": []byte(broken)})
			assert.NoError(t, err, "Check of broken content")
			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify after the check")
			assert.Empty(t, got.Items, "the errors of the disk")
		})

		t.Run("declines a change without a Go file", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, whole()).Check(t.Context(), map[source.Path][]byte{"notes.md": []byte("# notes\n")})
			assert.ErrorIs(t, err, engine.ErrDecline, "Check of a Markdown file")
		})

		t.Run("declines a file that no package compiles", func(t *testing.T) {
			t.Parallel()
			excluded := "//go:build ignore\n\npackage p\n\nvar X int = \"x\"\n"
			_, err := serving(t, whole()).Check(t.Context(), map[source.Path][]byte{"excluded.go": []byte(excluded)})
			assert.ErrorIs(t, err, engine.ErrDecline, "Check of a file that the build excludes")
			assert.Contains(t, err.Error(), "excluded.go", "the file in the reason")
		})
	})
}
