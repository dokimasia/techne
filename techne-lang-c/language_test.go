// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/c"
	"go.dokimi.dev/techne/lang/lsp"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language c", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Declaration().Language, c.Language, "the language of the declaration")
			assert.Equal(t, string(c.Language), "c", "the value of Language")
		})

		t.Run("claims the extensions of C", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Declaration().Extensions, []string{".c", ".h"}, "the extensions of C")
		})

		t.Run("lists the manifests of a C project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Declaration().Manifests, []string{
				"CMakeLists.txt", "GNUmakefile", "Makefile", "makefile", "meson.build",
				"compile_commands.json", "compile_flags.txt",
			}, "the manifests of C")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Declaration().Namespace("src/parser.c"), "src/parser", "the unit of src/parser.c")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "store"} {
				assert.Equal(t, c.Declaration().Visibility(name), sema.VisibilityUnknown, "the visibility of "+name)
			}
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs clangd without arguments", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Server().Command, []string{"clangd"}, "the command of clangd")
		})

		t.Run("opens a file as c", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Server().LanguageID, lsp.IdentityC, "the language identifier of clangd")
		})

		t.Run("claims no tier for check", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Server().Fidelity(engine.RoleCheck), trust.None, "the tier of clangd for check")
		})

		t.Run("claims Resolved for every other role of Binding", func(t *testing.T) {
			t.Parallel()
			for role := range lsp.Binding() {
				if role != engine.RoleCheck {
					assert.Equal(t, c.Server().Fidelity(role), trust.Resolved, "the tier of clangd for "+role.String())
				}
			}
		})
	})
}
