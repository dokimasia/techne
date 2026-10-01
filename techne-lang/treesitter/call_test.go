// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/treesitter"
)

// TestCall checks the contract of Calls that holds for every grammar. The conformance suite of
// each language module checks the calls of its fixture.
func TestCall(t *testing.T) {
	t.Parallel()

	t.Run("Calls", func(t *testing.T) {
		t.Parallel()

		t.Run("is a method of Engine", func(t *testing.T) {
			t.Parallel()
			calls := func(e *treesitter.Engine) func(context.Context, source.Path) ([]source.Span, error) {
				return e.Calls
			}
			assert.NotNil(t, calls, "the method Calls of Engine")
		})
	})
}
