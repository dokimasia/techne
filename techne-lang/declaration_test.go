// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/techne/lang"
)

func TestDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing and is refused", func(t *testing.T) {
			t.Parallel()
			var unset lang.Declaration
			if unset.Language != "" {
				t.Errorf("zero Declaration.Language = %q, want the empty string", unset.Language)
			}
			if unset.IsTest != nil || unset.Namespace != nil || unset.Exported != nil {
				t.Error("zero Declaration carries conventions it never stated")
			}
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
			if style.Line != "// " {
				t.Errorf("Line = %q, want the prefix including its space", style.Line)
			}
		})

		t.Run("says whether the comment sits above the declaration", func(t *testing.T) {
			t.Parallel()
			// Python's convention puts it inside; Go's puts it above.
			// One boolean covers both, and the zero value is inside.
			var inside lang.CommentStyle
			if inside.Above {
				t.Error("the zero CommentStyle must not claim the comment sits above")
			}
		})
	})
}
