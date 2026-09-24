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

// spanning returns a symbol with id over the bytes [start, end) of a/b.go.
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

func TestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Parents", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []sema.Symbol
			want []sema.ID
		}{
			{
				name: "links each symbol to its innermost container",
				give: []sema.Symbol{spanning("file", 0, 100), spanning("struct", 10, 60), spanning("field", 20, 30)},
				want: []sema.ID{"", "file", "struct"},
			},
			{
				name: "links no symbol to one it overlaps",
				give: []sema.Symbol{spanning("first", 0, 50), spanning("second", 40, 90)},
				want: []sema.ID{"", ""},
			},
			{
				name: "leaves the parent empty for a container with the same ID",
				give: []sema.Symbol{spanning("same", 0, 100), spanning("same", 10, 20)},
				want: []sema.ID{"", ""},
			},
			{
				name: "links no symbol to one with an equal span",
				give: []sema.Symbol{spanning("first", 0, 10), spanning("second", 0, 10)},
				want: []sema.ID{"", ""},
			},
			{
				name: "links no symbol to one in another file",
				give: []sema.Symbol{
					spanning("outer", 0, 100),
					{ID: "elsewhere", Span: source.Span{
						Path:  "c.go",
						Start: source.Position{Offset: 10},
						End:   source.Position{Offset: 20},
					}},
				},
				want: []sema.ID{"", ""},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				treesitter.Parents(tt.give)
				got := make([]sema.ID, len(tt.give))
				for i, s := range tt.give {
					got[i] = s.Parent
				}
				assert.Equal(t, got, tt.want, "parents")
			})
		}

		t.Run("accepts an empty slice", func(t *testing.T) {
			t.Parallel()
			assert.NotPanics(t, func() { treesitter.Parents(nil) }, "Parents")
		})
	})
}
