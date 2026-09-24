// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("implements Planner", func(t *testing.T) {
			t.Parallel()
			assert.True(t, is[engine.Planner]((*treesitter.Engine)(nil)), "Planner")
		})
	})
}
