// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestWord(t *testing.T) {
	t.Parallel()

	t.Run("Worded", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			text string
			word string
			want int
		}{
			{name: "returns the index of the name", text: "n := s.Get()", word: "Get", want: 7},
			{name: "skips the name inside a longer identifier", text: "s.GetAll(); s.Get()", word: "Get", want: 14},
			{name: "returns -1 for the name inside identifiers only", text: "s.GetAll(); x_Get", word: "Get", want: -1},
			{name: "returns -1 for an empty name", text: "n := s.Get()", want: -1},
			{name: "reads a letter outside ASCII as part of an identifier", text: "éGet Get", word: "Get", want: 6},
			{name: "finds an occurrence inside an occurrence it skips", text: "ba a a", word: "a a", want: 3},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Worded(tt.text, tt.word), tt.want, "Worded")
			})
		}
	})

	t.Run("WordAt", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			text   string
			offset int
			want   string
		}{
			{name: "returns the identifier that contains the offset", text: "s.Get()", offset: 3, want: "Get"},
			{name: "returns the identifier that ends at the offset", text: "s.Get()", offset: 5, want: "Get"},
			{name: "returns the empty string between two separators", text: "a  b", offset: 2},
			{name: "returns an identifier with letters outside ASCII", text: "x := café", offset: 6, want: "café"},
			{name: "returns the empty string for an offset past the text", text: "abc", offset: 4},
			{name: "returns the empty string for a negative offset", text: "abc", offset: -1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.WordAt(tt.text, tt.offset), tt.want, "WordAt")
			})
		}
	})
}
