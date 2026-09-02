// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

func TestApply(t *testing.T) {
	t.Parallel()

	t.Run("a write that fails partway", func(t *testing.T) {
		t.Parallel()

		t.Run("puts back what it had already written", func(t *testing.T) {
			t.Parallel()
			// Half a change compiles about as often as none of it, and
			// is far harder to find.
			files, s := serving(t, planner{also: "b.fx"}, clean())
			files.content["b.fx"] = original
			files.refuse = "b.fx"

			_, err := s.Apply(t.Context(), asking(false))
			assert.HasError(t, err, "a filesystem that will not take the write is a fault")
			assert.Equal(t, files.at("a.fx"), original,
				"the file that was written before the failure is put back")
			assert.Equal(t, files.at("b.fx"), original, "and the one that was not is untouched")
		})

		t.Run("says whether the workspace was put back", func(t *testing.T) {
			t.Parallel()
			// A caller told only that the write failed does not know
			// whether it is holding a workspace that was restored or one
			// left half changed.
			files, s := serving(t, planner{also: "b.fx"}, clean())
			files.content["b.fx"] = original
			files.refuse = "b.fx"

			_, err := s.Apply(t.Context(), asking(false))
			assert.Contains(t, err.Error(), "put back", "the failure says what state the workspace is in")
		})
	})

	t.Run("a plan that both rewrites a file and moves it", func(t *testing.T) {
		t.Parallel()

		t.Run("puts the rewrite at the destination", func(t *testing.T) {
			t.Parallel()
			// Moving a Java file renames the class inside it, so the
			// server answers with one change list holding both. Carrying
			// the file over as it was sealed would drop the rename,
			// silently and in the one language where it matters.
			files, s := serving(t, planner{moves: "b.fx"}, clean())
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a move the gate passed is applied")
			assert.True(t, got.Applied, "the workspace changed")
			assert.Equal(t, files.at("b.fx"), "// Doc.\none\ntwo\n",
				"the file arrived rewritten rather than as it was")
			assert.Equal(t, files.at("a.fx"), "", "and is gone from where it was")
		})

		t.Run("puts the file back when the write fails partway", func(t *testing.T) {
			t.Parallel()
			// A move is a remove and a write. Failing between them leaves
			// the workspace holding neither copy, which is the one
			// outcome worse than not moving it.
			files, s := serving(t, planner{moves: "b.fx"}, clean())
			files.refuse = "b.fx"

			_, err := s.Apply(t.Context(), asking(false))
			assert.HasError(t, err, "a filesystem that will not take the write is a fault")
			assert.Equal(t, files.at("a.fx"), original, "the file is back where it was")
			assert.Equal(t, files.at("b.fx"), "", "and nothing is at the destination")
		})
	})

	t.Run("a plan that names one move more than once", func(t *testing.T) {
		t.Parallel()

		t.Run("moves the file once", func(t *testing.T) {
			t.Parallel()
			// ruby-lsp answers a rename of a class with the file rename
			// repeated per site it found. Performed in turn, the second
			// reads what the first left behind — a path with nothing at
			// it — and the file is gone from both ends. Driving a rename
			// over a Ruby class lost the file.
			files, s := serving(t, planner{moves: "b.fx", twice: true}, clean())
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a move said twice is the move it describes")
			assert.True(t, got.Applied, "the workspace changed")
			assert.Equal(t, files.at("b.fx"), "// Doc.\none\ntwo\n",
				"the file arrived, with what the plan left in it")
			assert.Equal(t, files.at("a.fx"), "", "and is gone from where it was")
		})

		t.Run("refuses one that names two destinations", func(t *testing.T) {
			t.Parallel()
			// Two results rather than one said twice. Picking between
			// them is guessing which the planner meant.
			_, s := serving(t, planner{moves: "b.fx", astray: "c.fx"}, clean())
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a plan that cannot mean one thing is answerable")
			assert.Equal(t, got.Status, trust.Refused, "so it is refused rather than resolved")
			assert.Contains(t, got.Reason, "both", "and the reason names the two")
		})
	})

	t.Run("two callers changing one file", func(t *testing.T) {
		t.Parallel()

		t.Run("do not interleave", func(t *testing.T) {
			t.Parallel()
			// Locks are held from the content being pinned to the same
			// content being written back changed. Without them one
			// caller reads what the other is halfway through writing.
			_, s := serving(t, planner{}, clean())

			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					_, err := s.Apply(t.Context(), asking(false))
					assert.NoError(t, err, "every caller either writes or is refused, and none faults")
				})
			}
			wg.Wait()
		})
	})
}
