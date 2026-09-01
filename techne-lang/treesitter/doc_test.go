// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

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
					if string(c) == "definition."+named {
						t.Errorf("capture %q names a language", c)
					}
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
			if class != named {
				t.Errorf("class maps to %v and type to %v; a caller would read two sets", class, named)
			}
			if class != sema.KindType {
				t.Errorf("both map to %v, want KindType", class)
			}
		})
	})
}
