// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestOutline(t *testing.T) {
	t.Parallel()

	t.Run("the port", func(t *testing.T) {
		t.Parallel()

		t.Run("is served, and no other is claimed", func(t *testing.T) {
			t.Parallel()
			// What Outline returns for real source is asserted by the
			// conformance suite, which runs inside a language module
			// where a grammar exists. This module must not depend on
			// one, so what it can check here is the shape.
			var e engine.Engine = (*treesitter.Engine)(nil)
			_, serves := e.(engine.Outliner)
			assert.True(t, serves, "the engine serves the role its package exists for")
			assert.False(t, assertVerifier(e),
				"a parser cannot say what a compiler says, so it claims no role that needs one")
			assert.True(t, assertRelator(e),
				"one direction is written in the source rather than resolved from it, "+
					"and the relator declines every other")
			assert.True(t, assertIndexer(e),
				"an index can afford this engine, because a change to one file "+
					"cannot stale its facts about another")
			assert.True(t, assertPlanner(e),
				"one operation writes a comment above a declaration and needs nothing bound, "+
					"and the planner declines every other")
		})
	})
}

func assertRelator(e engine.Engine) bool  { _, ok := e.(engine.Relator); return ok }
func assertPlanner(e engine.Engine) bool  { _, ok := e.(engine.Planner); return ok }
func assertVerifier(e engine.Engine) bool { _, ok := e.(engine.Verifier); return ok }
func assertIndexer(e engine.Engine) bool  { _, ok := e.(engine.Indexer); return ok }
