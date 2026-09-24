// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestIndex(t *testing.T) {
	t.Parallel()

	var e engine.Indexer = (*treesitter.Engine)(nil)

	t.Run("Granularity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns InvalidateFile", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, e.Granularity(), engine.InvalidateFile, "Granularity")
		})
	})

	t.Run("Affected", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the changed path alone", func(t *testing.T) {
			t.Parallel()
			for _, changed := range []source.Path{"pkg/a.fx", "notes.md"} {
				assert.Equal(t, e.Affected(changed), []source.Path{changed}, string(changed))
			}
		})
	})
}
