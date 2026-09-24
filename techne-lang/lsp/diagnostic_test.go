// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestDiagnostic(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the diagnostics a server publishes", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Pushes, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server that publishes")
			assert.Length(t, got.Items, 3, "the published diagnostics of a.fake")
		})

		t.Run("grades each diagnostic as the server graded it", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify")
			assert.Equal(t, grades(got.Items),
				[]diag.Severity{diag.SeverityError, diag.SeverityWarning, diag.SeverityUnset},
				"the severities of the findings")
		})

		t.Run("returns the code of a diagnostic", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify")
			assert.Equal(t, got.Items[0].Diagnostic.Code, "E101", "the code of the first finding")
		})

		t.Run("returns the source of a diagnostic", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify")
			assert.Equal(t, got.Items[0].Diagnostic.Source, "fakecheck", "the source of the first finding")
		})

		t.Run("returns a numeric code as its digits", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify")
			assert.Equal(t, got.Items[1].Diagnostic.Code, "42", "the code of the second finding")
		})

		t.Run("returns the source line of a diagnostic", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify")
			assert.Equal(t, got.Items[0].Diagnostic.Snippet, "type Store struct {",
				"the snippet of the first finding")
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("declines content whose report the server has not published", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Pushes, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify opens a.fake")

			_, err = e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Faulty)})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})
	})
}

// grades returns the severity of each finding, in order.
func grades(findings []edit.Finding) []diag.Severity {
	out := make([]diag.Severity, 0, len(findings))
	for _, one := range findings {
		out = append(out, one.Diagnostic.Severity)
	}
	return out
}
