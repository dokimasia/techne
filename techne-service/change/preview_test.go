// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
)

func TestPreview(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the text that an edit writes with its file", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Rewrites, []edit.Rewrite{{Path: "a.fx", Line: 1, Now: "// Doc.\n"}},
				"the rewrites of the outcome")
		})

		t.Run("returns the text that an edit replaces", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{changes: []edit.Change{replacing("a.fx", 4, 7, "three")}}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Rewrites, []edit.Rewrite{{Path: "a.fx", Line: 2, Was: "two", Now: "three"}},
				"the rewrites of the outcome")
		})

		t.Run("returns the rewrites of a refused change", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, marking("// Doc."))
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.False(t, got.Applied, "the application of the change")
			assert.Length(t, got.Rewrites, 1, "the rewrites of the outcome")
		})

		t.Run("returns no rewrite for a created file", func(t *testing.T) {
			t.Parallel()
			made := []edit.Change{{Kind: edit.ChangeCreate, Path: "new.fx", Content: []byte("made\n")}}
			_, s := serving(t, planner{changes: made}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Empty(t, got.Rewrites, "the rewrites of the outcome")
			assert.Equal(t, got.Changes, made, "the changes of the outcome")
		})
	})
}
