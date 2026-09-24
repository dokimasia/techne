// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/typescript"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language typescript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Declaration().Language, typescript.Language, "the language of the declaration")
			assert.Equal(t, string(typescript.Language), "typescript", "the value of Language")
		})

		t.Run("claims the extensions of TypeScript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Declaration().Extensions, []string{".ts", ".mts", ".cts", ".tsx"},
				"the extensions of TypeScript")
		})

		t.Run("lists the manifests of a TypeScript project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Declaration().Manifests, []string{"package.json", "tsconfig.json"},
				"the manifests of TypeScript")
		})

		t.Run("reads test files by the rule of JavaScriptTest", func(t *testing.T) {
			t.Parallel()
			for _, p := range []string{"src/store.spec.ts", "test/app.e2e-spec.ts", "src/store.ts"} {
				assert.Equal(t, typescript.Declaration().IsTest(p), lang.JavaScriptTest(p), "the test status of "+p)
			}
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Declaration().Namespace("src/store.ts"), "src/store", "the unit of src/store.ts")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "store"} {
				assert.Equal(t, typescript.Declaration().Visibility(name), sema.VisibilityUnknown,
					"the visibility of "+name)
			}
		})
	})

	t.Run("Grammar", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a .tsx file with the TSX grammar", func(t *testing.T) {
			t.Parallel()
			g := typescript.Grammar()
			assert.NotNil(t, g.Dialects[".tsx"], "the grammar of .tsx")
			assert.True(t, g.For("src/view.tsx") == g.Dialects[".tsx"], "the grammar of src/view.tsx")
			assert.True(t, g.For("src/store.ts") == g.Language, "the grammar of src/store.ts")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs typescript-language-server over stdio", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Server().Command, []string{"typescript-language-server", "--stdio"},
				"the command of typescript-language-server")
		})

		t.Run("opens a file as typescript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Server().LanguageID, lsp.IdentityTypeScript,
				"the language identifier of typescript-language-server")
		})

		t.Run("opens a .tsx file as typescriptreact", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Server().Dialects, map[string]string{".tsx": lsp.IdentityTypeScriptReact},
				"the dialects of typescript-language-server")
		})

		t.Run("prefers the extraction of a method", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Server().Extracts, lsp.Refactor{
				Kind:   "refactor.extract.function",
				Titles: []string{"method in class", "function in module scope"},
			}, "the extraction of typescript-language-server")
		})

		t.Run("opens the files that write a name before a rename", func(t *testing.T) {
			t.Parallel()
			assert.True(t, typescript.Server().Scoped, "the scope of typescript-language-server")
		})
	})

	t.Run("Native", func(t *testing.T) {
		t.Parallel()

		t.Run("runs tsc over LSP on stdio", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Native().Command, []string{"tsc", "--lsp", "--stdio"}, "the command of tsc")
		})

		t.Run("passes Server.Valid", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, typescript.Native().Valid(), "Valid of tsc")
		})

		t.Run("declares no extraction", func(t *testing.T) {
			t.Parallel()
			assert.False(t, typescript.Native().Extracts.Offered(), "the extraction of tsc")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		workspace := func(json string) fstest.MapFS {
			return fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte(json)}}
		}
		tests := []struct {
			name  string
			files fstest.MapFS
			want  string
		}{
			{
				name:  "returns tsc for a workspace on TypeScript 7",
				files: workspace(`{"devDependencies": {"typescript": "^7.0.2"}}`),
				want:  "tsc",
			},
			{
				name:  "returns typescript-language-server for a workspace on TypeScript 6",
				files: workspace(`{"devDependencies": {"typescript": "^6.0.3"}}`),
				want:  "typescript-language-server",
			},
			{
				name:  "returns typescript-language-server for a workspace without TypeScript",
				files: workspace(`{"dependencies": {}}`),
				want:  "typescript-language-server",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, typescript.For(tt.files).Name, tt.want, "the server of the workspace")
			})
		}
	})
}
