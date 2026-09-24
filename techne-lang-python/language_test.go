// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/python"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language python", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Declaration().Language, python.Language, "the language of the declaration")
			assert.Equal(t, string(python.Language), "python", "the value of Language")
		})

		t.Run("claims the extensions of Python", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Declaration().Extensions, []string{".py", ".pyi"}, "the extensions of Python")
		})

		t.Run("lists the manifests of a Python project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Declaration().Manifests,
				[]string{"pyproject.toml", "setup.py", "setup.cfg", "pyrightconfig.json"}, "the manifests of Python")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Declaration().Namespace("pkg/store.py"), "pkg/store", "the unit of pkg/store.py")
		})

		t.Run("reads visibility from a leading underscore", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Declaration().Visibility("_helper"), sema.Unexported, "the visibility of _helper")
			assert.Equal(t, python.Declaration().Visibility("helper"), sema.Exported, "the visibility of helper")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs pyright-langserver over stdio", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Server().Command, []string{"pyright-langserver", "--stdio"},
				"the command of pyright")
		})

		t.Run("opens a file as python", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Server().LanguageID, lsp.IdentityPython, "the language identifier of pyright")
		})
	})
}
