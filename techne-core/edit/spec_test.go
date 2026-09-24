// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestSpec(t *testing.T) {
	t.Parallel()

	t.Run("SpecFor", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spec of every declared operation", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, declared := edit.SpecFor(op)
				assert.True(t, declared, string(op))
				assert.Equal(t, spec.Operation, op, string(op))
			}
		})

		t.Run("returns false for an undeclared operation", func(t *testing.T) {
			t.Parallel()
			_, declared := edit.SpecFor(edit.Operation("rename.everything"))
			assert.False(t, declared, "declared")
		})

		t.Run("returns at least one target kind for every operation", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				assert.NotEmpty(t, spec.Accepts, string(op))
			}
		})

		t.Run("returns a minimum above None for every operation", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				assert.NotEqual(t, spec.MinFidelity, trust.None, string(op))
			}
		})

		t.Run("requires Resolved for every reference rewrite", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				if spec, _ := edit.SpecFor(op); spec.RewritesReferences {
					assert.Equal(t, spec.MinFidelity, trust.Resolved, string(op))
				}
			}
		})

		t.Run("lists no key as both required and optional", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				for _, key := range spec.Required {
					assert.False(t, slices.Contains(spec.Optional, key), string(op)+" "+string(key))
				}
			}
		})
	})
}
