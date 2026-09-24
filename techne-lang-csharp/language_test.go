// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/csharp"
	"go.dokimi.dev/techne/lang/lsp"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language csharp", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Declaration().Language, csharp.Language, "the language of the declaration")
			assert.Equal(t, string(csharp.Language), "csharp", "the value of Language")
		})

		t.Run("claims the extension of C#", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Declaration().Extensions, []string{".cs"}, "the extensions of C#")
		})

		t.Run("lists the manifests of a C# project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Declaration().Manifests, []string{"*.csproj", "*.sln", "*.slnx"},
				"the manifests of C#")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Declaration().Namespace("src/Store.cs"), "src/Store", "the unit of src/Store.cs")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "store"} {
				assert.Equal(t, csharp.Declaration().Visibility(name), sema.VisibilityUnknown,
					"the visibility of "+name)
			}
		})

		t.Run("ignores the discard", func(t *testing.T) {
			t.Parallel()
			assert.True(t, csharp.Declaration().Blank["_"], "the blank identifiers of C#")
		})

		t.Run("documents inside a summary element", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Declaration().Comment.Document("Store keeps items.", "    "),
				"    /// <summary>\n    /// Store keeps items.\n    /// </summary>", "the documentation of C#")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs csharp-ls without arguments", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Server().Command, []string{"csharp-ls"}, "the command of csharp-ls")
		})

		t.Run("opens a file as csharp", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Server().LanguageID, lsp.IdentityCSharp, "the language identifier of csharp-ls")
		})

		t.Run("extracts a method by its title", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Server().Extracts, lsp.Refactor{Titles: []string{"extract method"}},
				"the extraction of csharp-ls")
		})
	})
}
