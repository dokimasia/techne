// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package python_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
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

		t.Run("declares that the server returns the imported file at an import", func(t *testing.T) {
			t.Parallel()
			assert.True(t, python.Server().Imports, "Imports of pyright")
		})

		t.Run("runs pyright-langserver over stdio", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Server().Command, []string{"pyright-langserver", "--stdio"},
				"the command of pyright")
		})

		t.Run("opens a file as python", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Server().LanguageID, lsp.IdentityPython, "the language identifier of pyright")
		})

		t.Run("declares that the server publishes the diagnostics of its open files alone", func(t *testing.T) {
			t.Parallel()
			assert.True(t, python.Server().OpenFilesOnly, "OpenFilesOnly of pyright")
		})

		t.Run("declares that the server restarts after a change of a manifest", func(t *testing.T) {
			t.Parallel()
			assert.True(t, python.Server().Restarts, "Restarts of pyright")
		})

		tests := []struct {
			name     string
			giveTree files.Tree // the workspace under workspace/ and the other files beside it
			giveEnv  string     // the directory that VIRTUAL_ENV names, or empty for none
			want     string     // the interpreter that the settings name, or empty for no settings
		}{
			{
				name:     "configures the interpreter of the .venv directory of the workspace",
				giveTree: files.Tree{"workspace/.venv/bin/python": files.Text("")},
				want:     "workspace/.venv/bin/python",
			},
			{
				name:     "configures the interpreter of a .venv directory in the layout of Windows",
				giveTree: files.Tree{"workspace/.venv/Scripts/python.exe": files.Text("")},
				want:     "workspace/.venv/Scripts/python.exe",
			},
			{
				name: "keeps the path of an interpreter that is a symbolic link",
				giveTree: files.Tree{
					"workspace/.venv/bin/python": files.Link("../../../base/python3"),
					"base/python3":               files.Text(""),
				},
				want: "workspace/.venv/bin/python",
			},
			{
				name:     "does not configure an interpreter for a symbolic link to a missing file",
				giveTree: files.Tree{"workspace/.venv/bin/python": files.Link("../../../base/python3")},
			},
			{
				name:     "configures the interpreter of VIRTUAL_ENV for a workspace without a .venv directory",
				giveTree: files.Tree{"workspace/main.py": files.Text(""), "env/bin/python": files.Text("")},
				giveEnv:  "env",
				want:     "env/bin/python",
			},
			{
				name: "configures the interpreter of the .venv directory in place of VIRTUAL_ENV",
				giveTree: files.Tree{
					"workspace/.venv/bin/python": files.Text(""), "env/bin/python": files.Text(""),
				},
				giveEnv: "env",
				want:    "workspace/.venv/bin/python",
			},
			{
				name:     "does not configure an interpreter for a workspace without a virtual environment",
				giveTree: files.Tree{"workspace/main.py": files.Text("")},
			},
			{
				name:     "does not configure an interpreter for a .venv directory without an interpreter",
				giveTree: files.Tree{"workspace/.venv/bin/pip": files.Text("")},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				dir := files.Workspace(t, tt.giveTree)
				getenv := func(key string) string {
					if key != "VIRTUAL_ENV" || tt.giveEnv == "" {
						return ""
					}
					return filepath.Join(dir, filepath.FromSlash(tt.giveEnv))
				}
				var want map[string]any
				if tt.want != "" {
					interpreter := filepath.Join(dir, filepath.FromSlash(tt.want))
					want = map[string]any{"python": map[string]any{"pythonPath": interpreter}}
				}
				got := python.Server().Configure(filepath.Join(dir, "workspace"), getenv)
				assert.Equal(t, got, want, "the settings of the workspace")
			})
		}
	})
}
