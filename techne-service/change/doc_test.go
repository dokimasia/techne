// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/change"
)

// TestDoc covers the claims the package comment makes.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the gate", func(t *testing.T) {
		t.Parallel()

		t.Run("runs before the write, so a refused change touches no file", func(t *testing.T) {
			t.Parallel()
			// RFC-0004 draws the gate after the apply because a build
			// gate needs files on disk. A gate that reads content does
			// not, and running it first means there is nothing to undo.
			files, s := serving(t, planner{}, faults("this does not parse"))
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a gate that objects is an answer")
			assert.Equal(t, files.at("a.fx"), original,
				"the file was never written, so it was never put back")
			assert.Equal(t, got.Status, trust.Refused, "and the caller is told why")
		})
	})

	t.Run("a dry run", func(t *testing.T) {
		t.Parallel()

		t.Run("promises what an apply does, byte for byte", func(t *testing.T) {
			t.Parallel()
			// A preview that only prints a diff answers "what would
			// change". This answers "would it still parse", which means
			// an agent can send a preview and an apply without reading
			// what came back in between.
			files, s := serving(t, planner{}, clean())

			previewed, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "previewing succeeds")
			applied, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "applying succeeds")

			assert.Equal(t, applied.Rewrites, previewed.Rewrites,
				"a dry run reports the edits the real call applies")
			assert.Equal(t, files.at("a.fx"), previewed.Rewrites[0].Now+original,
				"and the projection it gated is what landed")
		})
	})

	t.Run("a refusal", func(t *testing.T) {
		t.Parallel()

		t.Run("is a result, because a caller can act on one", func(t *testing.T) {
			t.Parallel()
			// An error means the workspace could not be read or written,
			// which is not something the caller got wrong.
			for _, refused := range []func() (*held, *change.Service){
				func() (*held, *change.Service) { return serving(t, planner{refuses: "no"}) },
				func() (*held, *change.Service) { return serving(t, planner{}, faults("broken")) },
			} {
				_, s := refused()
				got, err := s.Apply(t.Context(), asking(false))
				assert.NoError(t, err, "being refused is an answer, not a fault")
				assert.False(t, got.Applied, "and nothing was written")
			}
		})
	})
}
