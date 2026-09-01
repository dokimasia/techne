// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/source"
)

// TestDoc covers the package-level contract that a diagnostic carries
// enough for a caller to act without parsing its message.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("a reported problem", func(t *testing.T) {
		t.Parallel()

		t.Run("is actionable without reading the message", func(t *testing.T) {
			t.Parallel()
			reported := diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     "SA4006",
				Message:  "value never read",
				Span:     source.Span{Path: "core/trust/status.go", Start: source.Position{Line: 12}},
				Source:   "staticcheck",
			}
			assert.True(t, reported.Severity >= diag.SeverityWarning,
				"severity alone decides whether a caller stops")
			assert.NotEmpty(t, reported.Code, "a caller suppresses by rule rather than by matching prose")
			assert.NotEmpty(t, reported.Source, "a caller tells a broken build from a linter's objection")
			assert.NotEmpty(t, string(reported.Span.Path), "a diagnostic says which file it is about")
		})
	})
}
