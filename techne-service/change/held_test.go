// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"testing"

	"go.dokimi.dev/assert"
)

func TestHeld(t *testing.T) {
	t.Parallel()

	t.Run("a preview", func(t *testing.T) {
		t.Parallel()

		t.Run("comes back with a handle to apply it by", func(t *testing.T) {
			t.Parallel()
			// The plan stays here. What crosses the wire is a name, so a
			// caller never has to reproduce bytes exactly.
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "previewing succeeds")
			assert.NotEmpty(t, got.Handle, "a preview that could be applied says how")
			assert.False(t, got.Applied, "and has still written nothing")
		})

		t.Run("gets a handle nobody could have guessed", func(t *testing.T) {
			t.Parallel()
			// A handle is the authority to write a change somebody else
			// previewed, so a counter would let a caller apply a plan it
			// never saw.
			_, s := serving(t, planner{}, clean())
			seen := map[string]bool{}
			for range 8 {
				got, err := s.Apply(t.Context(), asking(true))
				assert.NoError(t, err, "previewing succeeds")
				assert.False(t, seen[got.Handle], "no two previews share a handle")
				assert.Length(t, got.Handle, 32, "sixteen bytes of hex")
				seen[got.Handle] = true
			}
		})

		t.Run("carries no handle once it has been applied", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "applying succeeds")
			assert.True(t, got.Applied, "the change was written")
			assert.Empty(t, got.Handle, "so there is nothing left to apply")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("writes what the preview described", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "previewing succeeds")

			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "applying a held plan succeeds")
			assert.True(t, got.Applied, "the change was written")
			assert.Equal(t, got.Rewrites, preview.Rewrites,
				"and it is the change the preview showed")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "which is what landed")
		})

		t.Run("refuses a handle nobody holds", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Commit(t.Context(), "0123456789abcdef0123456789abcdef")

			assert.NoError(t, err, "a handle nobody holds is correctable, not a fault")
			assert.False(t, got.Applied, "and nothing was written")
			assert.Contains(t, got.Reason, "preview again", "the caller is told what to do")
		})

		t.Run("refuses a handle twice", func(t *testing.T) {
			t.Parallel()
			// Applying is not something to do twice by accident.
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "previewing succeeds")

			_, err = s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "the first apply succeeds")
			again, err := s.Commit(t.Context(), preview.Handle)

			assert.NoError(t, err, "a spent handle is correctable")
			assert.False(t, again.Applied, "the change was not written a second time")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "so the comment landed once")
		})

		t.Run("refuses a plan whose file moved on", func(t *testing.T) {
			t.Parallel()
			// Byte ranges over other bytes describe other code, and the
			// result usually still compiles. The digest is what catches
			// it, and this is the call it was written for.
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "previewing succeeds")

			assert.NoError(t, files.Write("a.fx", []byte("somebody else was here\n")),
				"the file changes under the plan")

			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "drift is correctable, not a fault")
			assert.False(t, got.Applied, "the stale change was not written")
			assert.Contains(t, got.Reason, "no longer there", "and the caller is told why")
			assert.Equal(t, files.at("a.fx"), "somebody else was here\n",
				"leaving what the other writer put there")
		})

		t.Run("keeps only so many previews", func(t *testing.T) {
			t.Parallel()
			// A session previews far more often than it applies, and a
			// plan nobody came back for is one the caller changed its
			// mind about.
			_, s := serving(t, planner{}, clean())
			first, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "previewing succeeds")

			for range 32 {
				_, more := s.Apply(t.Context(), asking(true))
				assert.NoError(t, more, "previewing succeeds")
			}

			got, err := s.Commit(t.Context(), first.Handle)
			assert.NoError(t, err, "a dropped handle is correctable")
			assert.False(t, got.Applied, "the oldest plan went to make room")
		})
	})
}
