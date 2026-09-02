// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
)

func TestPreview(t *testing.T) {
	t.Parallel()

	t.Run("a change", func(t *testing.T) {
		t.Parallel()

		t.Run("is read back against the file it was computed on", func(t *testing.T) {
			t.Parallel()
			// A plan says which bytes move. Whoever reviews a change
			// reads what goes and what arrives, and the write path is
			// the only place both are known: it read the file and the
			// planner did not.
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "previewing succeeds")
			assert.Length(t, got.Rewrites, 1, "one edit reads back as one rewrite")
			assert.Equal(t, got.Rewrites[0].Now, "// Doc.\n", "the text that arrives")
			assert.Equal(t, string(got.Rewrites[0].Path), "a.fx", "and the file it arrives in")
		})

		t.Run("counts its line from one", func(t *testing.T) {
			t.Parallel()
			// Spans count from zero and editors count from one. A
			// preview is read by whoever opens the file.
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "previewing succeeds")
			assert.Equal(t, got.Rewrites[0].Line, 1,
				"an edit at the first byte is on line one, not line zero")
		})

		t.Run("carries nothing replaced for an insertion", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "previewing succeeds")
			assert.Empty(t, got.Rewrites[0].Was,
				"an insertion replaces nothing, and says so by carrying nothing")
		})
	})

	t.Run("a refused change", func(t *testing.T) {
		t.Parallel()

		t.Run("is still read back, so the caller sees what was refused", func(t *testing.T) {
			t.Parallel()
			// Being told a change was refused without being shown it
			// leaves a caller guessing at what to correct.
			_, s := serving(t, planner{}, faults("this does not parse"))
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a refusal is an answer")
			assert.False(t, got.Applied, "and nothing was written")
			assert.Length(t, got.Rewrites, 1, "the caller reads the change that was refused")
		})
	})

	t.Run("a change that is not an edit", func(t *testing.T) {
		t.Parallel()

		t.Run("reads back as nothing", func(t *testing.T) {
			t.Parallel()
			// Creating, deleting and moving a file are what the change
			// itself says they are. Putting a whole file's bytes in a
			// preview would cost more than reading the file.
			_, s := serving(t, planner{makes: "new.fx"}, clean())
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "previewing succeeds")
			for _, one := range got.Rewrites {
				assert.NotEqual(t, string(one.Path), "new.fx",
					"a created file is named by the change rather than quoted whole")
			}
			assert.True(t, slices.Contains(kinds(got.Changes), edit.ChangeCreate),
				"and the change still says the file is made")
		})
	})
}

// kinds is what a set of changes does, for a case asserting on the mix.
func kinds(changes []edit.Change) []edit.ChangeKind {
	out := make([]edit.ChangeKind, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.Kind)
	}
	return out
}
