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

	t.Run("the port", func(t *testing.T) {
		t.Parallel()

		t.Run("is served, so a change can be gated before it is written", func(t *testing.T) {
			t.Parallel()
			// What Check reports for real content is asserted by the
			// conformance suite, which runs where a grammar exists. What
			// is checked here is that the engine claims the role: a
			// language whose parse gate went missing would write changes
			// nothing had judged, and say they were verified.
			var e engine.Engine = (*treesitter.Engine)(nil)
			_, serves := e.(engine.Checker)
			assert.True(t, serves, "a parser can say whether content is still the language it was")
		})

		t.Run("is not the port that says whether it builds", func(t *testing.T) {
			t.Parallel()
			// The two are separate because an engine can serve either
			// without the other, and because "it parses" and "it builds"
			// are different promises. A parser claiming the second would
			// have every caller reading the weaker one as the stronger.
			var e engine.Engine = (*treesitter.Engine)(nil)
			_, builds := e.(engine.Verifier)
			assert.False(t, builds, "a parser gates content and cannot compile it")
		})
	})
}
