// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"
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
