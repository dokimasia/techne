// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package core_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// module is the import path of the core module.
const module = "go.dokimi.dev/techne/core"

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Dependency position", func(t *testing.T) {
		t.Parallel()
		listed := packages(t)

		t.Run("imports only the standard library and core", func(t *testing.T) {
			t.Parallel()
			for _, p := range listed {
				for _, imported := range p.Imports {
					first, _, _ := strings.Cut(imported, "/")
					standard := !strings.Contains(first, ".")
					assert.True(t, standard || imported == module || strings.HasPrefix(imported, module+"/"),
						p.ImportPath+" imports "+imported)
				}
			}
		})

		t.Run("imports no cgo", func(t *testing.T) {
			t.Parallel()
			for _, p := range listed {
				assert.Empty(t, p.CgoFiles, "the cgo files of "+p.ImportPath)
			}
		})
	})
}

// listed is one package as go list -json reports it.
type listed struct {
	ImportPath string
	Imports    []string
	CgoFiles   []string
}

// packages returns every package of the module as the go command lists it,
// with the imports of its production files.
func packages(t *testing.T) []listed {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-json", "./...").Output()
	assert.NoError(t, err, "go list of the module")
	var all []listed
	for decoder := json.NewDecoder(bytes.NewReader(out)); decoder.More(); {
		var one listed
		assert.NoError(t, decoder.Decode(&one), "the output of go list")
		all = append(all, one)
	}
	assert.NotEmpty(t, all, "the packages of the module")
	return all
}
