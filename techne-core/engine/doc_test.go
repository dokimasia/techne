// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// TestDoc covers the contract the package comment states across the
// ports and the engine interface together.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("fidelity is per role", func(t *testing.T) {
		t.Parallel()

		t.Run("so one engine can bind strongly and outline weakly", func(t *testing.T) {
			t.Parallel()
			// A language server resolves references through a type
			// system while returning an outline its own parser produced.
			// One tier per engine would force it to claim the weaker of
			// the two for both.
			var e engine.Engine = graded{}
			assert.True(t, e.Fidelity(engine.RoleRelate) > e.Fidelity(engine.RoleOutline),
				"one tier per engine would force a server to claim the weaker of what it does for both")
		})
	})
}

// graded stands for a language server: strong on references, weaker on
// document outline.
type graded struct{ outlineOnly }

func (graded) Fidelity(r engine.Role) trust.Fidelity {
	if r == engine.RoleRelate {
		return trust.Resolved
	}
	return trust.Syntactic
}
