// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package version_test

import (
	"encoding/json"
	"os/exec"
	"testing"

	"go.dokimi.dev/assert"
)

// TestDoc covers the claims of the package comment about the imports.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Dependency position", func(t *testing.T) {
		t.Parallel()

		t.Run("imports no package", func(t *testing.T) {
			t.Parallel()
			out, err := exec.CommandContext(t.Context(), "go", "list", "-json", ".").Output()
			assert.NoError(t, err, "go list of the package")
			var listed struct{ Imports []string }
			assert.NoError(t, json.Unmarshal(out, &listed), "the output of go list")
			assert.Empty(t, listed.Imports, "the imports of the package")
		})
	})
}
