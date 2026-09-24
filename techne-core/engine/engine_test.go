// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("ErrDecline", func(t *testing.T) {
		t.Parallel()

		t.Run("matches through a wrapping error", func(t *testing.T) {
			t.Parallel()
			wrapped := fmt.Errorf("checker: %w", declining("not loaded"))
			assert.ErrorIs(t, wrapped, engine.ErrDecline, "wrapped")
		})

		t.Run("differs from ErrRefuse", func(t *testing.T) {
			t.Parallel()
			assert.ErrorIsNot(t, engine.ErrRefuse, engine.ErrDecline, "ErrRefuse")
		})
	})

	t.Run("ErrRefuse", func(t *testing.T) {
		t.Parallel()

		t.Run("stops Ask at the refusing engine", func(t *testing.T) {
			t.Parallel()
			calls := 0
			c := catalog(t,
				fake{
					name: "checker", fidelity: trust.Resolved,
					err: fmt.Errorf("%w: rename across modules", engine.ErrRefuse),
				},
				fake{name: "parser", fidelity: trust.Syntactic, calls: &calls})
			_, _, _, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.ErrorIs(t, err, engine.ErrRefuse, "Ask")
			assert.Equal(t, calls, 0, "parser calls")
		})
	})
}
