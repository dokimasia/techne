// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
)

func TestTarget(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("points at nothing", func(t *testing.T) {
			t.Parallel()
			var unset edit.Target
			assert.Equal(t, unset.Kind, edit.TargetUnset,
				"a request naming no target is refused rather than served against zero fields")
			assert.Empty(t, string(unset.Symbol), "an unset target names no symbol")
			assert.Empty(t, string(unset.Path), "an unset target names no file")
		})

		t.Run("is not a kind any operation accepts", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				for _, k := range spec.Accepts {
					assert.NotEqual(t, k, edit.TargetUnset,
						"an operation accepting the unset kind would admit a request that named nothing")
				}
			}
		})
	})
}
