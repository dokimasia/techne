// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

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
			// The spec's minimum is necessary and not sufficient: an
			// engine reporting resolved binding over a partial scope
			// found some of the references, not all of them.
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if !spec.RewritesReferences {
					continue
				}
				if trust.SupportsNegativeClaim(spec.MinFidelity, trust.ScopePartial) {
					t.Errorf("operation %q would be admitted over a partial scope", op)
				}
				if !trust.SupportsNegativeClaim(spec.MinFidelity, trust.ScopeTotal) {
					t.Errorf("operation %q could never be admitted at all", op)
				}
			}
		})
	})

	t.Run("the catalogue", func(t *testing.T) {
		t.Parallel()

		t.Run("declares operations no language need serve", func(t *testing.T) {
			t.Parallel()
			// Nothing here consults a language. A caller is told a
			// language cannot do this, which needs the operation to
			// exist independently of any planner.
			if len(edit.Operations()) == 0 {
				t.Fatal("the catalogue is empty")
			}
			for _, op := range edit.Operations() {
				if _, ok := edit.SpecFor(op); !ok {
					t.Errorf("operation %q cannot be reported as unsupported: it has no spec", op)
				}
			}
		})
	})
}
