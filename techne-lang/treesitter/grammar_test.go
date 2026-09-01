// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestGrammar(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("declares neither a language nor a query", func(t *testing.T) {
			t.Parallel()
			// Both are required. A zero Grammar reaching New must fail
			// there rather than crashing on the first parse.
			var unset treesitter.Grammar
			assert.Nil(t, unset.Language, "an unset grammar carries no compiled language")
			assert.Empty(t, unset.Tags, "an unset grammar carries no query")
		})
	})
}
