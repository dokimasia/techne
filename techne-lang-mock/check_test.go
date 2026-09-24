// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"bytes"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/mock"
)

// broken returns a workspace with a use of a name that no declaration has.
func broken() fstest.MapFS {
	return fstest.MapFS{"a.mock": {Data: []byte("func Uses\n  use Absent\n")}}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no finding for content of the language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Check(t.Context(), map[source.Path][]byte{
				"a.mock": []byte(";; A store.\ntype Store\n  use Store\n\n"),
			})
			assert.NoError(t, err, "Check of a.mock")
			assert.Empty(t, got.Items, "the findings of a.mock")
		})

		t.Run("returns an error for a line that the language does not have", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Check(t.Context(), map[source.Path][]byte{
				"a.mock": []byte("type Store\n)]}%\n"),
			})
			assert.NoError(t, err, "Check of a.mock")
			assert.Length(t, got.Items, 1, "the findings of a.mock")
			found := got.Items[0].Diagnostic
			assert.Equal(t, found.Severity, diag.SeverityError, "the severity of the finding")
			assert.Equal(t, found.Code, "syntax", "the code of the finding")
			assert.Equal(t, found.Span, source.Span{
				Path:  "a.mock",
				Start: source.Position{Offset: 11, Line: 1},
				End:   source.Position{Offset: 15, Line: 1, Column: 4},
			}, "the span of the finding")
			assert.Equal(t, found.Snippet, ")]}%", "the snippet of the finding")
			assert.Empty(t, got.Items[0].Fix, "the fix of the finding")
		})

		t.Run("returns the findings in path order", func(t *testing.T) {
			t.Parallel()
			files := map[source.Path][]byte{}
			for _, p := range []source.Path{"f.mock", "e.mock", "d.mock", "c.mock", "b.mock", "a.mock"} {
				files[p] = []byte("not this language\n")
			}
			got, err := built(t).Check(t.Context(), files)
			assert.NoError(t, err, "Check of six files")
			var paths []source.Path
			for _, one := range got.Items {
				paths = append(paths, one.Diagnostic.Span.Path)
			}
			assert.True(t, slices.IsSorted(paths), "the order of the findings")
		})

		t.Run("declines a change without a file of the language", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Check(t.Context(), map[source.Path][]byte{"a.other": []byte("x")})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check of a.other")
		})

		t.Run("returns no finding for a file that a change deletes", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Check(t.Context(), map[source.Path][]byte{"a.mock": nil})
			assert.NoError(t, err, "Check of the deletion of a.mock")
			assert.Empty(t, got.Items, "the findings of the deletion")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a use of a name that no declaration has", func(t *testing.T) {
			t.Parallel()
			got, err := over(t, broken()).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of the workspace")
			assert.Equal(t, got.Items, []edit.Finding{{Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     "unresolved",
				Message:  "no declaration of the workspace is named Absent",
				Span: source.Span{
					Path:  "a.mock",
					Start: source.Position{Offset: 16, Line: 1, Column: 6},
					End:   source.Position{Offset: 22, Line: 1, Column: 12},
				},
				Source:  "mock/mock",
				Snippet: "use Absent",
			}}}, "the findings of the workspace")
		})

		t.Run("returns no finding for a use of a declaration outside the scope", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Verify(t.Context(), engine.Request{Scope: "src/client.mock"}, nil)
			assert.NoError(t, err, "Verify of src/client.mock")
			assert.Empty(t, got.Items, "the findings of src/client.mock")
		})

		t.Run("runs its check for every list of suites", func(t *testing.T) {
			t.Parallel()
			for _, suites := range [][]string{nil, {mock.Suite}, {"lint"}} {
				got, err := over(t, broken()).Verify(t.Context(), engine.Request{Scope: "."}, suites)
				assert.NoError(t, err, "Verify of the workspace")
				assert.Length(t, got.Items, 1, "the findings of the workspace")
			}
		})

		t.Run("returns a caveat that lists the suites that the language does not run", func(t *testing.T) {
			t.Parallel()
			got, err := over(t, broken()).Verify(t.Context(), engine.Request{Scope: "."},
				[]string{"lint", mock.Suite, "vet"})
			assert.NoError(t, err, "Verify of the workspace")
			assert.Equal(t, got.Caveats[len(got.Caveats)-1], trust.Caveat{
				Code: trust.CaveatUnsupported,
				Note: "the mock language runs its one check, resolve, and none of these suites: lint, vet",
			}, "the caveat of the suites")
		})

		t.Run("returns a skipped result for a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Verify(t.Context(), engine.Request{Scope: "notes.md"}, nil)
			assert.NoError(t, err, "Verify of notes.md")
			assert.True(t, got.Skipped, "the skip of notes.md")
		})

		t.Run("returns a partial answer for a workspace with a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			fsys := broken()
			fsys["big.mock"] = &fstest.MapFile{Data: bytes.Repeat([]byte("x"), lang.Largest+1)}
			got, err := over(t, fsys).Verify(t.Context(), engine.Request{Scope: "a.mock"}, nil)
			assert.NoError(t, err, "Verify of a.mock")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.Equal(t, got.Caveats[len(got.Caveats)-1].Paths, []source.Path{"big.mock"},
				"the files that the answer leaves out")
		})
	})
}
