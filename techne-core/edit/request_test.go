// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestRequest(t *testing.T) {
	t.Parallel()

	t.Run("Request", func(t *testing.T) {
		t.Parallel()

		t.Run("names an undeclared operation when zero", func(t *testing.T) {
			t.Parallel()
			var zero edit.Request
			_, declared := edit.SpecFor(zero.Operation)
			assert.False(t, declared, "declared")
		})
	})

	t.Run("Outcome", func(t *testing.T) {
		t.Parallel()

		t.Run("reports nothing applied when zero", func(t *testing.T) {
			t.Parallel()
			var zero edit.Outcome
			assert.False(t, zero.Applied, "Applied")
			assert.Empty(t, zero.Changed, "Changed")
			assert.Equal(t, zero.Status, trust.Unset, "Status")
		})
	})
}
