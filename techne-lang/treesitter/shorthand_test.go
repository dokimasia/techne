// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/treesitter"
)

// TestShorthand checks the contract of Shorthands that applies to every grammar. The conformance
// suite of each language module checks the shorthand properties of its fixture.
func TestShorthand(t *testing.T) {
	t.Parallel()

	t.Run("Shorthands", func(t *testing.T) {
		t.Parallel()

		t.Run("is a method of Engine", func(t *testing.T) {
			t.Parallel()
			shorthands := func(e *treesitter.Engine) func(context.Context, source.Path) ([]source.Span, error) {
				return e.Shorthands
			}
			assert.NotNil(t, shorthands, "the method Shorthands of Engine")
		})
	})
}
