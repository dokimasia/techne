// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
)

func TestLock(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("serialises the changes of one file", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			var wg sync.WaitGroup
			for range 16 {
				wg.Go(func() {
					_, err := s.Apply(t.Context(), asking(false))
					assert.NoError(t, err, "Apply")
				})
			}
			wg.Wait()
			assert.Equal(t, strings.Count(files.at("a.fx"), "// Doc.\n"), 16, "the comments in a.fx")
			assert.HasSuffix(t, files.at("a.fx"), original, "the content of a.fx")
		})

		t.Run("writes the concurrent changes of different files", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			paths := []source.Path{"a.fx", "b.fx", "c.fx", "d.fx"}
			for _, p := range paths[1:] {
				files.put(p, original)
			}
			var wg sync.WaitGroup
			for _, p := range paths {
				wg.Go(func() {
					req := asking(false)
					req.Scope, req.Target.Span.Path = p, p
					_, err := s.Apply(t.Context(), req)
					assert.NoError(t, err, "Apply to "+string(p))
				})
			}
			wg.Wait()
			for _, p := range paths {
				assert.Equal(t, files.at(p), "// Doc.\n"+original, "the content of "+string(p))
			}
		})

		t.Run("completes concurrent changes of two files", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: twice()}, clean())
			files.put("b.fx", original)
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					_, err := s.Apply(t.Context(), asking(false))
					assert.NoError(t, err, "Apply")
				})
			}
			wg.Wait()
			assert.Equal(t, strings.Count(files.at("b.fx"), "// Doc.\n"), 8, "the comments in b.fx")
		})
	})
}
