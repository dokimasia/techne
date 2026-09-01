// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/treesitter"
)

// TestDoc covers the claim the package comment makes: one engine serves
// many grammars because it knows a query convention rather than a
// language.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the convention", func(t *testing.T) {
		t.Parallel()

		t.Run("names no language", func(t *testing.T) {
			t.Parallel()
			// Were a capture named for one grammar, that grammar's
			// module would own it and this engine would stop being
			// shared.
			for _, c := range treesitter.Definitions() {
				for _, named := range []string{
					"c", "csharp", "go", "java", "javascript",
					"python", "ruby", "rust", "scala", "typescript",
				} {
					assert.NotEqual(t, string(c), "definition."+named,
						"a capture naming one grammar would belong to that grammar's module")
				}
			}
		})

		t.Run("holds what a query cannot capture outside the query", func(t *testing.T) {
			t.Parallel()
			// Nesting is decided by the bytes a declaration covers, not
			// by a pattern saying what encloses what, so it holds for a
			// grammar nobody has written a pattern for yet.
			symbols := []sema.Symbol{
				spanning("outer", 0, 50),
				spanning("inner", 10, 20),
			}
			treesitter.Parents(symbols)
			assert.Equal(t, string(symbols[1].Parent), "outer",
				"containment is read off the spans, so it needs nothing from the query")
		})

		t.Run("gives a caller one set whichever grammar answered", func(t *testing.T) {
			t.Parallel()
			// A Java class and a Go struct are one shape: a named
			// aggregate of fields and methods. They map to one kind so a
			// caller searching for that shape need not know which
			// language answered.
			class, _ := treesitter.KindOf(treesitter.DefinitionClass)
			shaped, _ := treesitter.KindOf(treesitter.DefinitionStruct)
			assert.Equal(t, class, shaped,
				"a caller reads one vocabulary rather than a variant per grammar")
			assert.Equal(t, class, sema.KindStruct,
				"both are named aggregates the shared set calls a struct")
		})
	})
}
