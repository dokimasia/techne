// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/treesitter"
)

// spanning builds a symbol covering one byte range of one file, which is
// all Parents reads.
func spanning(id sema.ID, start, end int) sema.Symbol {
	return sema.Symbol{
		ID: id,
		Span: source.Span{
			Path:  "a/b.go",
			Start: source.Position{Offset: start},
			End:   source.Position{Offset: end},
		},
	}
}

func TestParents(t *testing.T) {
	t.Parallel()

	t.Run("links a symbol to the innermost one containing it", func(t *testing.T) {
		t.Parallel()
		// A field inside a struct inside a file: the struct is nearer
		// than anything enclosing it, so the struct is the parent.
		symbols := []sema.Symbol{
			spanning("file", 0, 100),
			spanning("struct", 10, 60),
			spanning("field", 20, 30),
		}
		treesitter.Parents(symbols)

		assert.Equal(t, string(symbols[2].Parent), "struct",
			"the smallest span strictly containing a symbol is its parent")
		assert.Equal(t, string(symbols[1].Parent), "file",
			"the smallest span strictly containing a symbol is its parent")
	})

	t.Run("leaves the outermost symbol at the top level", func(t *testing.T) {
		t.Parallel()
		symbols := []sema.Symbol{
			spanning("outer", 0, 100),
			spanning("inner", 10, 20),
		}
		treesitter.Parents(symbols)
		assert.Empty(t, string(symbols[0].Parent),
			"a symbol nothing contains sits at the top level of its unit")
	})

	t.Run("does not link two symbols that only overlap", func(t *testing.T) {
		t.Parallel()
		// Overlap without containment is not nesting. Linking them would
		// build a tree the source does not have.
		symbols := []sema.Symbol{
			spanning("first", 0, 50),
			spanning("second", 40, 90),
		}
		treesitter.Parents(symbols)
		assert.Empty(t, string(symbols[0].Parent),
			"containment is nesting; overlap is not")
		assert.Empty(t, string(symbols[1].Parent),
			"containment is nesting; overlap is not")
	})

	t.Run("does not make a symbol its own parent", func(t *testing.T) {
		t.Parallel()
		// A grammar can nest one declaration inside another of the same
		// name and kind, which gives them one identity. A caller walking
		// parents would then loop.
		symbols := []sema.Symbol{
			spanning("same", 0, 100),
			spanning("same", 10, 20),
		}
		treesitter.Parents(symbols)
		assert.Empty(t, string(symbols[1].Parent),
			"a symbol that is its own parent would make a caller building a tree loop")
	})

	t.Run("links equal spans in one direction only", func(t *testing.T) {
		t.Parallel()
		// Two patterns can report one declaration over the same bytes.
		// Neither contains the other more tightly, so neither is nested.
		symbols := []sema.Symbol{
			spanning("first", 0, 10),
			spanning("second", 0, 10),
		}
		treesitter.Parents(symbols)
		assert.Empty(t, string(symbols[0].Parent),
			"two symbols covering the same bytes are siblings, not a chain")
		assert.Empty(t, string(symbols[1].Parent),
			"two symbols covering the same bytes are siblings, not a chain")
	})

	t.Run("accepts an empty slice", func(t *testing.T) {
		t.Parallel()
		var none []sema.Symbol
		treesitter.Parents(none)
		assert.Empty(t, none, "a file declaring nothing has nothing to link")
	})
}
