// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/internal/app"
	"go.dokimi.dev/techne/lang"
)

// imported are the packages outside the standard library that the package comment states the
// package imports.
var imported = []string{
	"go.dokimi.dev/techne/core/engine",
	"go.dokimi.dev/techne/core/source",
	"go.dokimi.dev/techne/core/trust",
	"go.dokimi.dev/techne/lang",
	"go.dokimi.dev/techne/lang/c",
	"go.dokimi.dev/techne/lang/csharp",
	"go.dokimi.dev/techne/lang/go",
	"go.dokimi.dev/techne/lang/java",
	"go.dokimi.dev/techne/lang/javascript",
	"go.dokimi.dev/techne/lang/mock",
	"go.dokimi.dev/techne/lang/python",
	"go.dokimi.dev/techne/lang/ruby",
	"go.dokimi.dev/techne/lang/rust",
	"go.dokimi.dev/techne/lang/scala",
	"go.dokimi.dev/techne/lang/typescript",
	"go.dokimi.dev/techne/presenter",
	"go.dokimi.dev/techne/service/change",
	"go.dokimi.dev/techne/service/query",
	"go.dokimi.dev/techne/service/workspace/files",
	"go.dokimi.dev/techne/tool",
}

// languageModule reports whether the import path is a language module, such as
// go.dokimi.dev/techne/lang/go, or the mock language.
func languageModule(path string) bool {
	name, found := strings.CutPrefix(path, "go.dokimi.dev/techne/lang/")
	return found && (slices.Contains(modules, source.Language(name)) || name == "mock")
}

// listed is one package as go list -json reports it.
type listed struct {
	ImportPath string
	Imports    []string
}

// packages returns the packages of the patterns as go list reports them, with the imports of
// their production files.
func packages(t *testing.T, patterns ...string) []listed {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", append([]string{"list", "-json"}, patterns...)...).Output()
	assert.NoError(t, err, "go list of "+strings.Join(patterns, " "))
	var all []listed
	for decoder := json.NewDecoder(bytes.NewReader(out)); decoder.More(); {
		var one listed
		assert.NoError(t, decoder.Decode(&one), "the output of go list")
		all = append(all, one)
	}
	assert.NotEmpty(t, all, "the packages of "+strings.Join(patterns, " "))
	return all
}

// TestDoc covers the claims of the package comment about the languages and the imports.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Languages", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the ten languages for a workspace without files", func(t *testing.T) {
			t.Parallel()
			s, err := app.Build(lang.Workspace{FS: fstest.MapFS{}}, nil, "")
			assert.NoError(t, err, "the error of Build")
			assert.Equal(t, languages(s), modules, "the languages of the server")
		})

		t.Run("is the only package of techne that imports a language module", func(t *testing.T) {
			t.Parallel()
			var importers []string
			for _, one := range packages(t, "go.dokimi.dev/techne/...") {
				if slices.ContainsFunc(one.Imports, languageModule) {
					importers = append(importers, one.ImportPath)
				}
			}
			assert.Equal(t, importers, []string{"go.dokimi.dev/techne/internal/app"},
				"the importers of a language module")
		})
	})

	t.Run("Dependency position", func(t *testing.T) {
		t.Parallel()

		t.Run("imports no package that the comment does not name", func(t *testing.T) {
			t.Parallel()
			for _, one := range packages(t, ".")[0].Imports {
				first, _, _ := strings.Cut(one, "/")
				standard := !strings.Contains(first, ".")
				assert.True(t, standard || slices.Contains(imported, one), "the package imports "+one)
			}
		})

		t.Run("is imported by the command techne alone", func(t *testing.T) {
			t.Parallel()
			var importers []string
			for _, one := range packages(t, "go.dokimi.dev/techne/...") {
				if slices.Contains(one.Imports, "go.dokimi.dev/techne/internal/app") {
					importers = append(importers, one.ImportPath)
				}
			}
			assert.Equal(t, importers, []string{"go.dokimi.dev/techne/cmd/techne"}, "the importers of the package")
		})
	})
}
