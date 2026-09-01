// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"testing"

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
			for i := 1; i < len(ordered); i++ {
				if ordered[i-1] >= ordered[i] {
					t.Errorf("severity %d is not below %d", ordered[i-1], ordered[i])
				}
			}
		})

		t.Run("lets a caller filter with one comparison", func(t *testing.T) {
			t.Parallel()
			// The ordering exists so that asking for errors alone does
			// not mean enumerating every other value.
			for _, s := range []diag.Severity{diag.SeverityHint, diag.SeverityInfo, diag.SeverityWarning} {
				if s >= diag.SeverityError {
					t.Errorf("severity %d sorts at or above SeverityError", s)
				}
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("carries no severity", func(t *testing.T) {
			t.Parallel()
			var unset diag.Diagnostic
			if unset.Severity != diag.SeverityUnset {
				t.Errorf("zero Diagnostic.Severity = %d, want SeverityUnset", unset.Severity)
			}
		})

		t.Run("names no reporting tool", func(t *testing.T) {
			t.Parallel()
			// Source distinguishes a broken build from a linter, so an
			// unattributed diagnostic must not read as either.
			var unset diag.Diagnostic
			if unset.Source != "" || unset.Code != "" {
				t.Errorf("zero Diagnostic attributes itself: %+v", unset)
			}
		})
	})
}
