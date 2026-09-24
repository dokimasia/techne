// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

func TestHeld(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a handle for a dry run", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Length(t, got.Handle, 32, "the handle of the outcome")
		})

		t.Run("returns a different handle for each dry run", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			seen := map[string]bool{}
			for range 8 {
				got, err := s.Apply(t.Context(), asking(true))
				assert.NoError(t, err, "Apply")
				assert.False(t, seen[got.Handle], "the reuse of "+got.Handle)
				seen[got.Handle] = true
			}
		})

		t.Run("returns no handle for a write", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Empty(t, got.Handle, "the handle of the outcome")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the change of the preview", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "Commit")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Rewrites, preview.Rewrites, "the rewrites of the commit")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("refuses a handle that no preview returned", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Commit(t.Context(), "0123456789abcdef0123456789abcdef")
			assert.NoError(t, err, "Commit")
			assert.False(t, got.Applied, "the application of the change")
			assert.Contains(t, got.Reason, "preview again", "the reason of the refusal")
		})

		t.Run("refuses a handle the second time", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			_, err = s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "the first Commit")
			again, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "the second Commit")
			assert.False(t, again.Applied, "the application of the second commit")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("refuses a plan whose file changed since the preview", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			files.put("a.fx", "somebody else\n")
			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "Commit")
			assert.False(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Reason, "a.fx changed after the plan read it, so the plan describes code "+
				"that the file does not contain: preview again", "the reason of the refusal")
			assert.Equal(t, files.at("a.fx"), "somebody else\n", "the content of a.fx")
		})

		t.Run("refuses a plan whose file was deleted since the preview", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.NoError(t, files.Remove("a.fx"), "Remove of a.fx")
			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "Commit")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Contains(t, got.Reason, "a.fx was deleted", "the reason of the refusal")
		})

		t.Run("refuses a create of a path that a file took since the preview", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: made()}, clean())
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			files.put("b.fx", "somebody else\n")
			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "Commit")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at("b.fx"), "somebody else\n", "the content of b.fx")
		})

		t.Run("gates the commit with the language of the preview", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			files.put("a.txt", original)
			req := asking(true)
			req.Scope, req.Target.Span.Path = "a.txt", "a.txt"
			preview, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "Commit")
			assert.True(t, got.Applied, "the application of the change")
			assert.NotNil(t, got.Gate, "the gate of the commit")
			assert.Equal(t, got.Gate.Engine, "clean", "the engine of the gate")
		})

		t.Run("returns a degraded outcome for a commit that no engine checks", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			preview, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			got, err := s.Commit(t.Context(), preview.Handle)
			assert.NoError(t, err, "Commit")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Status, trust.Degraded, "the status of the commit")
		})

		t.Run("drops the oldest preview beyond 32", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			first, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "the first Apply")
			for range 32 {
				_, more := s.Apply(t.Context(), asking(true))
				assert.NoError(t, more, "Apply")
			}
			got, err := s.Commit(t.Context(), first.Handle)
			assert.NoError(t, err, "Commit")
			assert.False(t, got.Applied, "the application of the oldest preview")
		})
	})
}
