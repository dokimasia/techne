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
			for name, claimed := range map[string]bool{
				"Searcher": assertSearcher(e),
				"Relator":  assertRelator(e),
				"Planner":  assertPlanner(e),
				"Verifier": assertVerifier(e),
			} {
				_ = name
				assert.False(t, claimed, "a parser cannot serve a role needing more than text, so it claims none")
			}
		})
	})
}

func assertSearcher(e engine.Engine) bool { _, ok := e.(engine.Searcher); return ok }
func assertRelator(e engine.Engine) bool  { _, ok := e.(engine.Relator); return ok }
func assertPlanner(e engine.Engine) bool  { _, ok := e.(engine.Planner); return ok }
func assertVerifier(e engine.Engine) bool { _, ok := e.(engine.Verifier); return ok }
