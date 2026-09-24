// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/test/corpus"
)

// imported are the packages outside the standard library that the package comment states the
// package imports.
var imported = []string{
	"go.dokimi.dev/techne/core/source",
	"go.dokimi.dev/techne/tool",
	"github.com/modelcontextprotocol/go-sdk/mcp",
}

// commit matches a full commit of git.
var commit = regexp.MustCompile(`^[0-9a-f]{40}$`)

// TestDoc covers the claims of the package comment about the repositories and the imports.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Repositories", func(t *testing.T) {
		t.Parallel()

		t.Run("pins each clone to a tag and a full commit", func(t *testing.T) {
			t.Parallel()
			m, err := corpus.Load("corpus.json")
			assert.NoError(t, err, "Load of corpus.json")
			for _, r := range m.Repositories {
				if !r.Writable() {
					continue
				}
				assert.NotEmpty(t, r.Tag, "the tag of "+r.Name)
				assert.True(t, commit.MatchString(r.Commit), "the commit of "+r.Name+": "+r.Commit)
			}
		})
	})

	t.Run("Dependency position", func(t *testing.T) {
		t.Parallel()

		t.Run("imports no package that the comment does not name", func(t *testing.T) {
			t.Parallel()
			out, err := exec.CommandContext(t.Context(), "go", "list", "-json", ".").Output()
			assert.NoError(t, err, "go list of the package")
			var listed struct{ Imports []string }
			assert.NoError(t, json.Unmarshal(out, &listed), "the output of go list")
			assert.NotEmpty(t, listed.Imports, "the imports of the package")
			for _, one := range listed.Imports {
				first, _, _ := strings.Cut(one, "/")
				standard := !strings.Contains(first, ".")
				assert.True(t, standard || slices.Contains(imported, one), "the package imports "+one)
			}
		})
	})
}
