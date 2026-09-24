// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package presenter_test

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// imported are the packages outside the standard library that the package comment states the
// presenter imports.
var imported = []string{"go.dokimi.dev/techne/tool", "github.com/modelcontextprotocol/go-sdk/mcp"}

// TestDoc covers the claims of the package comment about the results and the imports.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		t.Run("sends the payload as the tool encoded it", func(t *testing.T) {
			t.Parallel()
			log := &record{}
			called(t, connected(t, log), "outline", `{"scope":"a.fx"}`)
			assert.Contains(t, log.String(), `"structuredContent":{"status":"ok","scope":"a.fx"}`,
				"the result that the client read")
		})
	})

	t.Run("Dependency position", func(t *testing.T) {
		t.Parallel()

		t.Run("imports no package that the comment does not name", func(t *testing.T) {
			t.Parallel()
			out, err := exec.CommandContext(t.Context(), "go", "list", "-json", ".").Output()
			assert.NoError(t, err, "go list of the presenter")
			var listed struct{ Imports []string }
			assert.NoError(t, json.Unmarshal(out, &listed), "the output of go list")
			assert.NotEmpty(t, listed.Imports, "the imports of the presenter")
			for _, one := range listed.Imports {
				first, _, _ := strings.Cut(one, "/")
				standard := !strings.Contains(first, ".")
				assert.True(t, standard || slices.Contains(imported, one), "the presenter imports "+one)
			}
		})
	})
}
