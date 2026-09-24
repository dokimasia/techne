// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("implements Checker", func(t *testing.T) {
			t.Parallel()
			assert.True(t, is[engine.Checker]((*treesitter.Engine)(nil)), "Checker")
		})

		t.Run("does not implement Verifier", func(t *testing.T) {
			t.Parallel()
			assert.False(t, is[engine.Verifier]((*treesitter.Engine)(nil)), "Verifier")
		})
	})
}
