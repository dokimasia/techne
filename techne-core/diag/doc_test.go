// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Severity", func(t *testing.T) {
		t.Parallel()

		t.Run("filters errors with one comparison", func(t *testing.T) {
			t.Parallel()
			for _, s := range diag.Severities() {
				assert.Equal(t, s >= diag.SeverityError, s == diag.SeverityError, s.String())
			}
		})
	})
}
