// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/treesitter"
)

// is reports whether e implements the port P.
func is[P any](e engine.Engine) bool {
	_, ok := e.(P)
	return ok
}

func TestOutline(t *testing.T) {
	t.Parallel()

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("implements Outliner", func(t *testing.T) {
			t.Parallel()
			assert.True(t, is[engine.Outliner]((*treesitter.Engine)(nil)), "Outliner")
		})
	})
}
