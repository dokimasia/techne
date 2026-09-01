// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/techne/core/edit"
)

func TestTarget(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("points at nothing", func(t *testing.T) {
			t.Parallel()
			// A request that named no target must be refused, not served
			// against whatever the zero fields happen to mean.
			var unset edit.Target
			if unset.Kind != edit.TargetUnset {
				t.Errorf("zero Target.Kind = %d, want TargetUnset", unset.Kind)
			}
			if unset.Symbol != "" || unset.Path != "" {
				t.Errorf("zero Target names something: %+v", unset)
			}
		})

		t.Run("is not a kind any operation accepts", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				for _, k := range spec.Accepts {
					if k == edit.TargetUnset {
						t.Errorf("operation %q accepts TargetUnset, so an unset target would be admitted", op)
					}
				}
			}
		})
	})
}
