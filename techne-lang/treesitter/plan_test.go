// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("the operation it serves", func(t *testing.T) {
		t.Parallel()

		t.Run("is the one a parser can be correct on", func(t *testing.T) {
			t.Parallel()
			// What Plan produces for real source is asserted by the
			// conformance suite, which runs inside a language module
			// where a grammar exists. This module must not depend on
			// one, so what it checks here is why that operation and no
			// other.
			//
			// Writing a comment onto a declaration needs the
			// declaration's position, the language's comment forms and
			// the text to write, and nothing about what any name means.
			spec, declared := edit.SpecFor(edit.DocumentSymbol)
			assert.True(t, declared, "the operation this engine serves is in the catalogue")
			assert.Equal(t, spec.MinFidelity, trust.Syntactic,
				"a parser can serve it, which is why this engine offers to")
			assert.False(t, spec.RewritesReferences,
				"and it changes nothing that refers to its target")
		})

		t.Run("is the only one whose minimum a parser meets", func(t *testing.T) {
			t.Parallel()
			// A plan built on matched text would be wrong in exactly the
			// cases nobody checks, so the engine declines the rest
			// rather than serving them badly.
			for _, op := range edit.Operations() {
				if op == edit.DocumentSymbol {
					continue
				}
				spec, _ := edit.SpecFor(op)
				assert.True(t, spec.MinFidelity > trust.Syntactic,
					"an operation a parser could serve and this engine declines would be a gap")
			}
		})
	})

	t.Run("what it points at", func(t *testing.T) {
		t.Parallel()

		t.Run("may be a span, because a name is not one declaration", func(t *testing.T) {
			t.Parallel()
			// A unit declaring two methods called Get satisfies one
			// identity twice, so a caller that has already resolved
			// which it means says so by position.
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			var takesSpan bool
			for _, kind := range spec.Accepts {
				takesSpan = takesSpan || kind == edit.TargetSpan
			}
			assert.True(t, takesSpan, "the engine can be pointed at one declaration unambiguously")
		})
	})
}
