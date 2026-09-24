// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestSearch(t *testing.T) {
	t.Parallel()

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("implements Searcher", func(t *testing.T) {
			t.Parallel()
			assert.True(t, is[engine.Searcher]((*treesitter.Engine)(nil)), "Searcher")
		})
	})
}
