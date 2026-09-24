// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestGrammar(t *testing.T) {
	t.Parallel()

	main, dialect := &ts.Language{}, &ts.Language{}
	g := treesitter.Grammar{Language: main, Dialects: map[string]*ts.Language{".tsx": dialect}}

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the dialect of the extension", func(t *testing.T) {
			t.Parallel()
			assert.True(t, g.For("src/app.tsx") == dialect, "grammar")
		})

		t.Run("returns Language for an extension without a dialect", func(t *testing.T) {
			t.Parallel()
			assert.True(t, g.For("src/app.ts") == main, "grammar")
		})
	})
}
