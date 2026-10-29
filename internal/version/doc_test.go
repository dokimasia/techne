// Copyright Dokimasia B.V. 2026
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

		t.Run("imports the standard library and two packages of golang.org/x/mod", func(t *testing.T) {
			t.Parallel()
			out, err := exec.CommandContext(t.Context(), "go", "list", "-json", ".").Output()
			assert.NoError(t, err, "go list of the package")
			var listed struct {
				Imports []string `json:"Imports"`
			}
			assert.NoError(t, json.Unmarshal(out, &listed), "the output of go list")
			assert.Equal(t, listed.Imports,
				[]string{"golang.org/x/mod/module", "golang.org/x/mod/semver", "runtime/debug", "strings"},
				"the imports of the package")
		})
	})
}
