// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

func TestLocks(t *testing.T) {
	t.Parallel()

	t.Run("one file", func(t *testing.T) {
		t.Parallel()

		t.Run("is changed by one caller at a time", func(t *testing.T) {
			t.Parallel()
			// Each caller prepends a line, so sixteen callers leave
			// sixteen lines only if each of them read what the one
			// before it wrote. A caller pinning the file while another
			// was writing would read stale content, and its line would
			// replace rather than follow.
			//
			// Run under -race, this is also what says the map behind the
			// locks is safe.
			files, s := serving(t, planner{}, clean())

			var wg sync.WaitGroup
			for range 16 {
				wg.Go(func() {
					_, err := s.Apply(t.Context(), asking(false))
					assert.NoError(t, err, "a caller waiting its turn still gets served")
				})
			}
			wg.Wait()

			assert.Equal(t, strings.Count(files.at("a.fx"), "// Doc.\n"), 16,
				"sixteen lines means every caller saw the one before it finish")
			assert.HasSuffix(t, files.at("a.fx"), original,
				"and none of them lost what was there to begin with")
		})
	})

	t.Run("different files", func(t *testing.T) {
		t.Parallel()

		t.Run("are changed without waiting for each other", func(t *testing.T) {
			t.Parallel()
			// Per path rather than per workspace. A single lock would
			// make every change wait for every other one's gate, which
			// for a real gate is seconds rather than milliseconds.
			files, s := serving(t, planner{}, clean())
			for _, p := range []source.Path{"b.fx", "c.fx", "d.fx"} {
				files.content[p] = original
			}

			var wg sync.WaitGroup
			for _, p := range []source.Path{"a.fx", "b.fx", "c.fx", "d.fx"} {
				wg.Go(func() {
					req := asking(false)
					req.Scope = p
					req.Target.Span.Path = p
					_, err := s.Apply(t.Context(), req)
					assert.NoError(t, err, "a change to its own file is served")
				})
			}
			wg.Wait()

			for _, p := range []source.Path{"a.fx", "b.fx", "c.fx", "d.fx"} {
				assert.Equal(t, files.at(p), "// Doc.\n"+original,
					"every file was written, and none blocked on another's lock")
			}
		})
	})

	t.Run("a change touching several files", func(t *testing.T) {
		t.Parallel()

		t.Run("takes them in the order the plan sorts them", func(t *testing.T) {
			t.Parallel()
			// Two changes touching the same files in different orders
			// would each hold what the other waits for. The order comes
			// from the plan, which sorts, so no caller chooses it.
			files, s := serving(t, planner{also: "b.fx"}, clean())
			files.content["b.fx"] = original

			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					_, err := s.Apply(t.Context(), asking(false))
					assert.NoError(t, err, "concurrent multi-file changes complete")
				})
			}
			wg.Wait()

			assert.Equal(t, edit.Plan{Changes: []edit.Change{
				{Kind: edit.ChangeEdit, Path: "b.fx"},
				{Kind: edit.ChangeEdit, Path: "a.fx"},
			}}.Paths(), []source.Path{"a.fx", "b.fx"},
				"the plan sorts its paths, which is what makes the order the same for everyone")
		})
	})
}
