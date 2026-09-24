// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	faulty := map[source.Path][]byte{"a.fake": []byte(lsptest.Faulty)}

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the error of content that is not on disk", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.Length(t, got.Items, 1, "the findings of faulty content")
			assert.Equal(t, got.Items[0].Diagnostic.Severity, diag.SeverityError, "the severity of the finding")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns nothing for content that compiles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).
				Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content)})
			assert.NoError(t, err, "Check of content that compiles")
			assert.Empty(t, got.Items, "the findings of content that compiles")
		})

		t.Run("returns the one fix the server offers", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.Length(t, got.Items[0].Fix, 1, "the changes of the fix")
			assert.Equal(t, got.Items[0].Fix[0].Edits[0].New, "declared", "the text of the fix")
		})

		t.Run("converts the fix against the checked content", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			at := got.Items[0].Fix[0].Edits[0].Span
			assert.Equal(t, lsptest.Faulty[at.Start.Offset:at.End.Offset], lsptest.Broken,
				"the text that the fix replaces")
		})

		t.Run("sends the content on disk again after the check", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Compiles, sample())
			_, err := e.Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the check")
			assert.Empty(t, got.Items, "the findings of the file on disk")
		})

		t.Run("declines content of another language", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Compiles, sample()).
				Check(t.Context(), map[source.Path][]byte{"notes.md": []byte("# notes\n")})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})

		t.Run("declines a change that deletes the file", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Compiles, sample()).
				Check(t.Context(), map[source.Path][]byte{"a.fake": nil})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})

		t.Run("declines content the server reports nothing about", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Ungated, sample()).Check(t.Context(), faulty)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})

		t.Run("declines while the server loads the workspace", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Stuck, sample()).Check(t.Context(), faulty)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
			assert.Contains(t, err.Error(), "loading", "the error of Check")
		})

		t.Run("returns an error in a file that depends on the change", func(t *testing.T) {
			t.Parallel()
			renamed := strings.Replace(lsptest.Content, "type Store", "type Vault", 1)
			got, err := serving(t, lsptest.WorkspaceDiagnostics, map[string]string{
				"a.fake": lsptest.Content, "b.fake": "var _ Store\n",
			}).Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(renamed)})
			assert.NoError(t, err, "Check of a rename that misses b.fake")
			assert.True(t, slices.ContainsFunc(got.Items, func(one edit.Finding) bool {
				return one.Diagnostic.Span.Path == "b.fake"
			}), "a finding in b.fake")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatDependents), "the answer has a dependents caveat")
		})

		t.Run("adds a dependents caveat for a server without workspace diagnostics", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatDependents), "the answer has a dependents caveat")
		})

		t.Run("adds a partial-check caveat for a server that leaves checks out", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Compiles)
			server.Unchecked = "lifetimes or borrows"
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})

		t.Run("adds no partial-check caveat for a server that checks what the compiler checks", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})
	})
}
