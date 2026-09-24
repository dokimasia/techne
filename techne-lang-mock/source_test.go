// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/mock"
)

// covered returns the text of content that span covers.
func covered(content []byte, span source.Span) string {
	return string(content[span.Start.Offset:span.End.Offset])
}

func TestSource(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a declaration with the documentation above it", func(t *testing.T) {
			t.Parallel()
			lines, broken := mock.Parse("a.mock", []byte(";; one\n;; two\ntype Store\n"))
			assert.Empty(t, broken, "the broken lines")
			assert.Length(t, lines, 1, "the lines")
			assert.Equal(t, lines[0].Name, "Store", "the name of the declaration")
			assert.Equal(t, lines[0].Kind, sema.KindType, "the kind of the declaration")
			assert.Equal(t, lines[0].Doc, "one\ntwo", "the documentation of the declaration")
		})

		t.Run("returns a use without a name", func(t *testing.T) {
			t.Parallel()
			lines, _ := mock.Parse("a.mock", []byte("func New\n  use Store\n"))
			assert.Equal(t, lines[1].Uses, "Store", "the name that the use names")
			assert.Empty(t, lines[1].Name, "the name of the use")
		})

		t.Run("drops the documentation above a blank line", func(t *testing.T) {
			t.Parallel()
			lines, _ := mock.Parse("a.mock", []byte(";; lost\n\ntype Store\n"))
			assert.Empty(t, lines[0].Doc, "the documentation of Store")
		})

		t.Run("returns the span of a line that the language does not have", func(t *testing.T) {
			t.Parallel()
			_, broken := mock.Parse("a.mock", []byte("type Store\nnot this language\n"))
			assert.Equal(t, broken, []source.Span{{
				Path:  "a.mock",
				Start: source.Position{Offset: 11, Line: 1},
				End:   source.Position{Offset: 28, Line: 1, Column: 17},
			}}, "the broken lines")
		})

		t.Run("returns a declaration with two spaces after the word as broken", func(t *testing.T) {
			t.Parallel()
			lines, broken := mock.Parse("a.mock", []byte("type  Store\n"))
			assert.Empty(t, lines, "the lines")
			assert.Length(t, broken, 1, "the broken lines")
		})

		t.Run("spans the name of a line alone", func(t *testing.T) {
			t.Parallel()
			content := []byte("type Store\n  use Store\n")
			lines, _ := mock.Parse("a.mock", content)
			for _, one := range lines {
				assert.Equal(t, covered(content, one.At), "Store",
					"the name on line "+strconv.Itoa(one.Span.Start.Line))
			}
		})

		t.Run("returns one level of depth per two spaces of indentation", func(t *testing.T) {
			t.Parallel()
			lines, _ := mock.Parse("a.mock", []byte("type Store\n  field size\n    use Other\n"))
			depths := []int{lines[0].Depth, lines[1].Depth, lines[2].Depth}
			assert.Equal(t, depths, []int{0, 1, 2}, "the depths of the lines")
		})

		t.Run("starts the span of a line after its indentation", func(t *testing.T) {
			t.Parallel()
			content := []byte("type Store\n  field size\n")
			lines, _ := mock.Parse("a.mock", content)
			assert.Equal(t, lines[1].Span, source.Span{
				Path:  "a.mock",
				Start: source.Position{Offset: 13, Line: 1, Column: 2},
				End:   source.Position{Offset: 23, Line: 1, Column: 12},
			}, "the span of the field")
			assert.Equal(t, covered(content, lines[1].Span), "field size", "the text of the field")
		})

		t.Run("ends a line at a carriage return before a line feed", func(t *testing.T) {
			t.Parallel()
			content := []byte("type Store\r\n  use Store\r\n")
			lines, broken := mock.Parse("a.mock", content)
			assert.Empty(t, broken, "the broken lines")
			assert.Equal(t, lines[0].Name, "Store", "the name of the declaration")
			assert.Equal(t, lines[1].Uses, "Store", "the name that the use names")
			for _, one := range lines {
				assert.Equal(t, covered(content, one.At), "Store",
					"the name on line "+strconv.Itoa(one.Span.Start.Line))
			}
		})
	})

	t.Run("Kinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns words that each open a declaration", func(t *testing.T) {
			t.Parallel()
			for _, word := range mock.Kinds() {
				lines, broken := mock.Parse("a.mock", []byte(word+" Name\n"))
				assert.Empty(t, broken, "the broken lines of "+word)
				assert.Equal(t, lines[0].Name, "Name", "the name that "+word+" declares")
				assert.NotEqual(t, lines[0].Kind, sema.KindUnknown, "the kind that "+word+" declares")
			}
		})

		t.Run("returns the words in byte order", func(t *testing.T) {
			t.Parallel()
			assert.True(t, slices.IsSorted(mock.Kinds()), "the order of the words")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want sema.Visibility
		}{
			{
				name: "returns exported for a name that starts with an upper-case letter",
				give: "Store",
				want: sema.Exported,
			},
			{
				name: "returns unexported for a name that starts with a lower-case letter",
				give: "size",
				want: sema.Unexported,
			},
			{name: "returns unknown for the empty name", give: "", want: sema.VisibilityUnknown},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, mock.Visibility(tt.give), tt.want, "the visibility of "+tt.give)
			})
		}
	})
}
