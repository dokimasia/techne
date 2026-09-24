// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
)

func TestDiagnostic(t *testing.T) {
	t.Parallel()

	words := map[diag.Severity]string{
		diag.SeverityUnset:   "unset",
		diag.SeverityHint:    "hint",
		diag.SeverityInfo:    "info",
		diag.SeverityWarning: "warning",
		diag.SeverityError:   "error",
	}

	t.Run("Diagnostic", func(t *testing.T) {
		t.Parallel()

		t.Run("has SeverityUnset when zero", func(t *testing.T) {
			t.Parallel()
			var zero diag.Diagnostic
			assert.Equal(t, zero.Severity, diag.SeverityUnset, "severity")
		})

		t.Run("names no source when zero", func(t *testing.T) {
			t.Parallel()
			var zero diag.Diagnostic
			assert.Empty(t, zero.Source, "source")
		})
	})

	t.Run("Severities", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every severity in increasing order", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, diag.Severities(), len(words), "severities")
			assert.Pairwise(t, diag.Severities(), func(lower, higher diag.Severity) bool {
				return lower < higher
			}, "severity order")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pinned string of every severity", func(t *testing.T) {
			t.Parallel()
			for s, want := range words {
				assert.Equal(t, s.String(), want, "wire string")
			}
		})

		t.Run("returns unset for an undeclared severity", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, diag.Severity(200).String(), "unset", "wire string")
		})
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips every severity", func(t *testing.T) {
			t.Parallel()
			for s, want := range words {
				encoded, err := json.Marshal(s)
				assert.NoError(t, err, "marshal")
				assert.Equal(t, string(encoded), `"`+want+`"`, "encoding")
				var decoded diag.Severity
				assert.NoError(t, json.Unmarshal(encoded, &decoded), "unmarshal")
				assert.Equal(t, decoded, s, "round trip")
			}
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes an unknown string to SeverityUnset", func(t *testing.T) {
			t.Parallel()
			got := diag.SeverityError
			assert.NoError(t, json.Unmarshal([]byte(`"fatal"`), &got), "unmarshal")
			assert.Equal(t, got, diag.SeverityUnset, "decoded severity")
		})
	})
}
