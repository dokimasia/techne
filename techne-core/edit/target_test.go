// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
)

func TestTarget(t *testing.T) {
	t.Parallel()

	t.Run("TargetUnset", func(t *testing.T) {
		t.Parallel()

		t.Run("is the kind of the zero Target", func(t *testing.T) {
			t.Parallel()
			var zero edit.Target
			assert.Equal(t, zero.Kind, edit.TargetUnset, "kind")
		})

		t.Run("is accepted by no operation", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				assert.False(t, slices.Contains(spec.Accepts, edit.TargetUnset), string(op))
			}
		})
	})
}
