// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestVendored(t *testing.T) {
	t.Parallel()

	t.Run("Vendored", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true for a dependency or tool directory", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{
				".git", "node_modules", "vendor", "target", ".venv",
				"__pycache__", "site-packages", ".bloop", ".gradle",
			} {
				assert.True(t, lang.Vendored(name), name)
			}
		})

		t.Run("returns false for a directory that contains source in some projects", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"build", "dist", "out", "bin", "obj", "lib", "src"} {
				assert.False(t, lang.Vendored(name), name)
			}
		})

		t.Run("returns false for a path", func(t *testing.T) {
			t.Parallel()
			assert.False(t, lang.Vendored("node_modules/react"), "path")
		})

		t.Run("returns false for an empty name", func(t *testing.T) {
			t.Parallel()
			assert.False(t, lang.Vendored(""), "empty name")
		})
	})
}
