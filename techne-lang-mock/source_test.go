// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/mock"
)

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("a line", func(t *testing.T) {
		t.Parallel()

		t.Run("declares, refers, documents or is blank", func(t *testing.T) {
			t.Parallel()
			lines, broken := mock.Parse("a.mock", []byte(
				";; what it is\ntype Store\n  use Other\n\n"))

			assert.Empty(t, broken, "four shapes, all of them this language")
			assert.Length(t, lines, 2, "documentation and blanks make no line of their own")
			assert.Equal(t, lines[0].Name, "Store", "a declaration names what it makes")
			assert.Equal(t, lines[0].Doc, "what it is", "and carries what was written above it")
			assert.Equal(t, lines[1].Uses, "Other", "a use names what it refers to")
			assert.Empty(t, lines[1].Name, "and declares nothing")
		})

		t.Run("is broken when it is none of them", func(t *testing.T) {
			t.Parallel()
			// A gate over this language is worth something only because
			// there is something for it to catch.
			_, broken := mock.Parse("a.mock", []byte("type Store\nnonsense here and there\n"))
			assert.Length(t, broken, 1, "a line that is not this language is reported")
			assert.Equal(t, broken[0].Start.Line, 1, "at the line it is on")
		})

		t.Run("is broken when it is lenient about spacing", func(t *testing.T) {
			t.Parallel()
			// The name's position is worked out from the two lengths, so
			// a parser that shrugged at extra spaces is one where a
			// rename writes over the wrong bytes.
			_, broken := mock.Parse("a.mock", []byte("type  Store\n"))
			assert.Length(t, broken, 1, "one word, one space, one name")
		})
	})

	t.Run("a name", func(t *testing.T) {
		t.Parallel()

		t.Run("is spanned on its own, not with the line around it", func(t *testing.T) {
			t.Parallel()
			// Rewriting the line would rewrite the keyword too, which is
			// the difference between a rename and a substitution.
			content := []byte("type Store\n  use Store\n")
			lines, _ := mock.Parse("a.mock", content)

			assert.Equal(t, string(content[lines[0].At.Start.Offset:lines[0].At.End.Offset]),
				"Store", "the declaration's name and nothing else")
			assert.Equal(t, string(content[lines[1].At.Start.Offset:lines[1].At.End.Offset]),
				"Store", "and the use's, wherever the indentation put it")
		})
	})

	t.Run("nesting", func(t *testing.T) {
		t.Parallel()

		t.Run("is two spaces to a level", func(t *testing.T) {
			t.Parallel()
			lines, _ := mock.Parse("a.mock", []byte("type Store\n  field size\n    use Other\n"))
			assert.Equal(t, lines[0].Depth, 0, "a declaration at the margin")
			assert.Equal(t, lines[1].Depth, 1, "one inside it")
			assert.Equal(t, lines[2].Depth, 2, "and one inside that")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is read off the name, as Go reads it", func(t *testing.T) {
			t.Parallel()
			// A language needs some rule, and this is the shortest to
			// state. What matters for the tools is that there is one.
			assert.Equal(t, mock.Visibility("Store"), sema.Exported, "a capital opens it up")
			assert.Equal(t, mock.Visibility("size"), sema.Unexported, "and lowercase closes it")
			assert.Equal(t, mock.Visibility(""), sema.VisibilityUnknown, "a name that is not one says nothing")
		})
	})
}
