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

	t.Run("the port", func(t *testing.T) {
		t.Parallel()

		t.Run("is served by the same engine that outlines", func(t *testing.T) {
			t.Parallel()
			// A search reads the same declarations an outline does, so
			// one engine serves both and a caller reaches the same
			// evidence either way. What it returns for real source is
			// asserted by the conformance suite, which runs inside a
			// language module where a grammar exists.
			var e engine.Engine = (*treesitter.Engine)(nil)

			_, searches := e.(engine.Searcher)
			assert.True(t, searches, "the engine serves the search role")

			_, outlines := e.(engine.Outliner)
			assert.True(t, outlines, "the same engine serves the outline role")
		})
	})
}
