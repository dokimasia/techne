// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("writes no file for a change that the gate refuses", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, marking("// Doc."))
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.writes("a.fx"), 0, "the writes of a.fx")
		})

		t.Run("writes the rewrites that a dry run of the change returns", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			previewed, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "the dry run")
			applied, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "the write")
			assert.Equal(t, applied.Rewrites, previewed.Rewrites, "the rewrites of the write")
			assert.Equal(t, files.at("a.fx"), previewed.Rewrites[0].Now+original, "the content of a.fx")
		})

		t.Run("returns a refusal without an error", func(t *testing.T) {
			t.Parallel()
			for _, engines := range [][]engine.Engine{
				{planner{refuses: "no"}},
				{planner{}, marking("// Doc.")},
			} {
				_, s := serving(t, engines...)
				got, err := s.Apply(t.Context(), asking(false))
				assert.NoError(t, err, "Apply")
				assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			}
		})
	})
}
