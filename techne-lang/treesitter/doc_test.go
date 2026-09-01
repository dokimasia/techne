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
				for _, named := range []string{"go", "python", "java", "rust", "typescript"} {
					assert.NotEqual(t, string(c), "definition."+named,
						"a capture naming one grammar would belong to that grammar's module")
				}
			}
		})

		t.Run("gives a caller one set whichever grammar answered", func(t *testing.T) {
			t.Parallel()
			// A class in one grammar and a type in another are the same
			// kind here, so a caller reads one vocabulary rather than
			// per-language variants.
			class, _ := treesitter.KindOf(treesitter.DefinitionClass)
			named, _ := treesitter.KindOf(treesitter.DefinitionType)
			assert.Equal(t, class, named,
				"a caller reads one vocabulary rather than a variant per grammar")
			assert.Equal(t, class, sema.KindType, "both are named product types the shared set calls a type")
		})
	})
}
