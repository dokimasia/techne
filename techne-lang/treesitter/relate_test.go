// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

// What Relate answers over real source is asserted in a language module,
// where a grammar exists and an import can actually be read. This module
// must not depend on one, so what it checks here is which directions the
// vocabulary offers and that the pair this engine serves is one.
func TestRelate(t *testing.T) {
	t.Parallel()

	t.Run("the direction it serves", func(t *testing.T) {
		t.Parallel()

		t.Run("is a pair the vocabulary declares", func(t *testing.T) {
			t.Parallel()
			// A service asks the direction a caller wanted and inverts
			// it for an engine that stores the other. Serving one half
			// of a pair the vocabulary does not carry would leave the
			// other half unaskable.
			held := sema.RelationKinds()
			assert.True(t, slices.Contains(held, sema.Imports),
				"what a file brings into scope")
			assert.True(t, slices.Contains(held, sema.ImportedBy),
				"and what brings it in")
			assert.Equal(t, sema.Imports.Inverse(), sema.ImportedBy,
				"asked from either end, it is the same edge")
		})

		t.Run("is the only one a parser can be correct about", func(t *testing.T) {
			t.Parallel()
			// An import is a declaration the tags query captures. Every
			// other direction is a binding: who calls this, what refers
			// to this, what implements this. A parser matched text, so
			// answering those would put a guess at the top of a
			// catalogue's order for a question only binding settles.
			for _, held := range []sema.RelationKind{
				sema.Calls, sema.CalledBy,
				sema.References, sema.ReferencedBy,
				sema.Implements, sema.ImplementedBy,
			} {
				assert.False(t, slices.Contains(served(), held),
					"needs a binding this tier does not have: "+held.String())
			}
		})

		t.Run("leaves out embedding, which no query captures", func(t *testing.T) {
			t.Parallel()
			// The vocabulary carries the direction and no language
			// module reads it. Declining is what says so; answering none
			// would be a claim that a type embeds nothing.
			for _, held := range []sema.RelationKind{sema.Embeds, sema.EmbeddedBy} {
				assert.False(t, slices.Contains(served(), held),
					"nothing here reads it: "+held.String())
			}
		})
	})
}

// served is the directions this engine answers, which is the pair an
// import is written as and nothing else.
func served() []sema.RelationKind {
	return []sema.RelationKind{sema.Imports, sema.ImportedBy}
}
