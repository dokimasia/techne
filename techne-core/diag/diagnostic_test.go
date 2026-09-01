// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
)

func TestDiagnostic(t *testing.T) {
	t.Parallel()

	t.Run("Severity", func(t *testing.T) {
		t.Parallel()

		t.Run("rises from hint to error", func(t *testing.T) {
			t.Parallel()
			ordered := []diag.Severity{
				diag.SeverityUnset, diag.SeverityHint, diag.SeverityInfo,
				diag.SeverityWarning, diag.SeverityError,
			}
			assert.Pairwise(t, ordered, func(earlier, later diag.Severity) bool {
				return earlier < later
			}, "the order rises, so a caller filters with one comparison")
		})

		t.Run("lets a caller filter with one comparison", func(t *testing.T) {
			t.Parallel()
			for _, s := range []diag.Severity{diag.SeverityHint, diag.SeverityInfo, diag.SeverityWarning} {
				assert.True(t, s < diag.SeverityError,
					"asking for errors alone does not mean enumerating every other value")
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("carries no severity", func(t *testing.T) {
			t.Parallel()
			var unset diag.Diagnostic
			assert.Equal(t, unset.Severity, diag.SeverityUnset,
				"a diagnostic nobody graded claims no severity")
		})

		t.Run("names no reporting tool", func(t *testing.T) {
			t.Parallel()
			var unset diag.Diagnostic
			assert.Empty(t, unset.Source, "an unattributed diagnostic reads as neither a build nor a linter")
			assert.Empty(t, unset.Code, "an unattributed diagnostic carries no rule to suppress by")
		})
	})
}
