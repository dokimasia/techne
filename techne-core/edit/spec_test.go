// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestSpec(t *testing.T) {
	t.Parallel()

	t.Run("SpecFor", func(t *testing.T) {
		t.Parallel()

		t.Run("answers for every declared operation", func(t *testing.T) {
			t.Parallel()
			// An operation declared without a spec fails open: the
			// service validates against nothing and admits whatever a
			// planner produced. This is what keeps the two lists
			// together.
			for _, op := range edit.Operations() {
				if _, ok := edit.SpecFor(op); !ok {
					t.Errorf("operation %q has no spec", op)
				}
			}
		})

		t.Run("reports an operation nobody declared", func(t *testing.T) {
			t.Parallel()
			if _, ok := edit.SpecFor(edit.Operation("rename.everything")); ok {
				t.Error("SpecFor accepted an operation that is not in the catalogue")
			}
		})

		t.Run("returns the spec of the operation asked for", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if spec.Operation != op {
					t.Errorf("SpecFor(%q) returned the spec of %q", op, spec.Operation)
				}
			}
		})
	})

	t.Run("Accepts", func(t *testing.T) {
		t.Parallel()

		t.Run("names at least one target kind", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if len(spec.Accepts) == 0 {
					t.Errorf("operation %q accepts no target kind, so nothing can invoke it", op)
				}
			}
		})
	})

	t.Run("MinFidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is a tier something can actually reach", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if spec.MinFidelity == trust.None {
					t.Errorf("operation %q declares no minimum fidelity", op)
				}
			}
		})

		t.Run("is syntactic only where nothing is rewritten but the target", func(t *testing.T) {
			t.Parallel()
			// document.symbol writes a comment above a declaration and
			// touches nothing else, which is why a parser can serve it.
			// Any other operation at that tier would be guessing.
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if spec.MinFidelity == trust.Syntactic && spec.RewritesReferences {
					t.Errorf("operation %q rewrites references from a parser's evidence", op)
				}
			}
		})
	})

	t.Run("RewritesReferences", func(t *testing.T) {
		t.Parallel()

		t.Run("demands resolved binding", func(t *testing.T) {
			t.Parallel()
			// Admission also demands total coverage, which is a property
			// of the plan rather than the spec. The spec rules out the
			// tiers that can never be enough.
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if spec.RewritesReferences && spec.MinFidelity != trust.Resolved {
					t.Errorf("operation %q rewrites references at fidelity %d; that needs resolved binding",
						op, spec.MinFidelity)
				}
			}
		})
	})
}
