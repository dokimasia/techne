// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"testing"

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
			if reported.Severity < diag.SeverityWarning {
				t.Error("severity must let a caller decide whether to stop")
			}
			if reported.Code == "" || reported.Source == "" {
				t.Error("code and source must let a caller suppress without matching prose")
			}
			if reported.Span.Path == "" {
				t.Error("a diagnostic must say which file it is about")
			}
		})
	})
}
