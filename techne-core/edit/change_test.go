// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
)

func TestChange(t *testing.T) {
	t.Parallel()

	t.Run("Change", func(t *testing.T) {
		t.Parallel()

		t.Run("is ChangeUnset when zero", func(t *testing.T) {
			t.Parallel()
			var zero edit.Change
			assert.Equal(t, zero.Kind, edit.ChangeUnset, "kind")
			assert.Nil(t, zero.Content, "content")
		})
	})

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			give  string
			edits []edit.TextEdit
			want  string
		}{
			{
				name:  "replaces disjoint ranges",
				give:  "hello world",
				edits: []edit.TextEdit{at(0, 5, "HELLO"), at(6, 11, "WORLD")},
				want:  "HELLO WORLD",
			},
			{
				name:  "inserts at an empty range",
				give:  "ab",
				edits: []edit.TextEdit{at(1, 1, "-")},
				want:  "a-b",
			},
			{
				name:  "applies inserts at one offset in list order",
				give:  "ab",
				edits: []edit.TextEdit{at(1, 1, "1"), at(1, 1, "2")},
				want:  "a12b",
			},
			{
				name:  "applies an insert before a replacement at the same offset",
				give:  "abc",
				edits: []edit.TextEdit{at(1, 1, "+"), at(1, 2, "B")},
				want:  "a+Bc",
			},
			{
				name:  "applies touching replacements",
				give:  "abcd",
				edits: []edit.TextEdit{at(0, 2, "X"), at(2, 4, "Y")},
				want:  "XY",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := edit.Apply([]byte(tt.give), tt.edits)
				assert.NoError(t, err, "apply")
				assert.Equal(t, string(got), tt.want, "result")
			})
		}

		t.Run("rejects an edit that starts inside the previous edit", func(t *testing.T) {
			t.Parallel()
			_, err := edit.Apply([]byte("abcdef"), []edit.TextEdit{at(0, 3, "x"), at(2, 4, "y")})
			assert.ErrorIs(t, err, edit.ErrDisordered, "overlapping edits")
		})

		t.Run("rejects a replacement followed by an insert at its start", func(t *testing.T) {
			t.Parallel()
			_, err := edit.Apply([]byte("abc"), []edit.TextEdit{at(1, 2, "B"), at(1, 1, "+")})
			assert.ErrorIs(t, err, edit.ErrDisordered, "insert after a replacement")
		})

		t.Run("rejects an edit that ends before it starts", func(t *testing.T) {
			t.Parallel()
			_, err := edit.Apply([]byte("abc"), []edit.TextEdit{at(2, 1, "x")})
			assert.ErrorIs(t, err, edit.ErrDisordered, "inverted range")
		})

		t.Run("rejects an edit past the end of the content", func(t *testing.T) {
			t.Parallel()
			_, err := edit.Apply([]byte("abc"), []edit.TextEdit{at(2, 9, "x")})
			assert.HasError(t, err, "edit past the end")
		})
	})
}
