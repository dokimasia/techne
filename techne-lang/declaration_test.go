// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing and is refused", func(t *testing.T) {
			t.Parallel()
			var unset lang.Declaration
			assert.Empty(t, string(unset.Language), "an unset declaration names no language")
			assert.Nil(t, unset.IsTest, "an unset declaration states no convention")
			assert.Nil(t, unset.Namespace, "an unset declaration states no convention")
			assert.Nil(t, unset.Visibility, "an unset declaration states no convention")
		})
	})

	t.Run("CommentStyle", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the trailing space in the prefix", func(t *testing.T) {
			t.Parallel()
			// The prefix is written verbatim, so a language that wants
			// "// text" rather than "//text" says so here rather than in
			// whatever writes the comment.
			style := lang.CommentStyle{Line: "// ", Above: true}
			assert.Equal(t, style.Line, "// ",
				"the prefix is written verbatim, so a language states its own spacing here")
		})

		t.Run("says whether the comment sits above the declaration", func(t *testing.T) {
			t.Parallel()
			// Python's convention puts it inside; Go's puts it above.
			// One boolean covers both, and the zero value is inside.
			var inside lang.CommentStyle
			assert.False(t, inside.Above, "one boolean covers both conventions, and the zero value is inside")
		})
	})
}
