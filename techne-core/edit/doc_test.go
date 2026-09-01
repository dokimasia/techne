// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// TestDoc covers the contracts the package comment states across the
// catalogue and the spec table together.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("an operation that rewrites references", func(t *testing.T) {
		t.Parallel()

		t.Run("cannot be admitted on binding alone", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if !spec.RewritesReferences {
					continue
				}
				assert.False(t, trust.SupportsNegativeClaim(spec.MinFidelity, trust.ScopePartial),
					"an engine resolving over a partial scope found some references, not all of them")
				assert.True(t, trust.SupportsNegativeClaim(spec.MinFidelity, trust.ScopeTotal),
					"the declared minimum has to be reachable, or the operation could never run")
			}
		})
	})

	t.Run("the catalogue", func(t *testing.T) {
		t.Parallel()

		t.Run("declares operations no language need serve", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, edit.Operations(), "a caller cannot be told what it may ask for from an empty catalogue")
			for _, op := range edit.Operations() {
				_, declared := edit.SpecFor(op)
				assert.True(t, declared,
					"reporting a language cannot do this needs the operation to exist without a planner")
			}
		})
	})
}
