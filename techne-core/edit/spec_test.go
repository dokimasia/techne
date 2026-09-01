// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestSpec(t *testing.T) {
	t.Parallel()

	t.Run("SpecFor", func(t *testing.T) {
		t.Parallel()

		t.Run("answers for every declared operation", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				_, declared := edit.SpecFor(op)
				assert.True(t, declared,
					"an operation without a spec is validated against nothing, "+
						"and admitted on whatever a planner produced")
			}
		})

		t.Run("reports an operation nobody declared", func(t *testing.T) {
			t.Parallel()
			_, declared := edit.SpecFor(edit.Operation("rename.everything"))
			assert.False(t, declared, "an operation outside the catalogue is not silently accepted")
		})

		t.Run("returns the spec of the operation asked for", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				assert.Equal(t, spec.Operation, op, "a spec describes the operation it was asked about")
			}
		})
	})

	t.Run("Accepts", func(t *testing.T) {
		t.Parallel()

		t.Run("names at least one target kind", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				assert.NotEmpty(t, spec.Accepts, "an operation nothing can be pointed at cannot be invoked")
			}
		})
	})

	t.Run("MinFidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is a tier something can actually reach", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				assert.NotEqual(t, spec.MinFidelity, trust.None,
					"an operation declaring no minimum could never be admitted")
			}
		})

		t.Run("is syntactic only where nothing but the target is rewritten", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if spec.MinFidelity != trust.Syntactic {
					continue
				}
				assert.False(t, spec.RewritesReferences,
					"a parser matches text, so it cannot find every reference to rewrite")
			}
		})
	})

	t.Run("RewritesReferences", func(t *testing.T) {
		t.Parallel()

		t.Run("demands resolved binding", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if !spec.RewritesReferences {
					continue
				}
				assert.Equal(t, spec.MinFidelity, trust.Resolved,
					"finding every reference is a claim no others exist, which needs a type checker")
			}
		})
	})
}
