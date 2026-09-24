// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript_test

import (
	"bytes"
	"fmt"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/javascript"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language javascript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Declaration().Language, javascript.Language, "the language of the declaration")
			assert.Equal(t, string(javascript.Language), "javascript", "the value of Language")
		})

		t.Run("claims the extensions of JavaScript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Declaration().Extensions, []string{".js", ".mjs", ".cjs", ".jsx"},
				"the extensions of JavaScript")
		})

		t.Run("lists the manifests of a JavaScript project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Declaration().Manifests, []string{"package.json", "jsconfig.json"},
				"the manifests of JavaScript")
		})

		t.Run("reads test files by the rule of JavaScriptTest", func(t *testing.T) {
			t.Parallel()
			for _, p := range []string{"src/store.test.js", "test/store.js", "src/store.js"} {
				assert.Equal(t, javascript.Declaration().IsTest(p), lang.JavaScriptTest(p), "the test status of "+p)
			}
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Declaration().Namespace("src/store.js"), "src/store", "the unit of src/store.js")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "store"} {
				assert.Equal(t, javascript.Declaration().Visibility(name), sema.VisibilityUnknown,
					"the visibility of "+name)
			}
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs typescript-language-server over stdio", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().Command, []string{"typescript-language-server", "--stdio"},
				"the command of typescript-language-server")
		})

		t.Run("opens a file as javascript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().LanguageID, lsp.IdentityJavaScript,
				"the language identifier of typescript-language-server")
		})

		t.Run("opens a .jsx file as javascriptreact", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().Dialects, map[string]string{".jsx": lsp.IdentityJavaScriptReact},
				"the dialects of typescript-language-server")
		})

		t.Run("prefers the extraction of a method", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().Extracts, lsp.Refactor{
				Kind:   "refactor.extract.function",
				Titles: []string{"method in class", "function in module scope"},
			}, "the extraction of typescript-language-server")
		})

		t.Run("opens the files that write a name before a rename", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Server().Scoped, "the scope of typescript-language-server")
		})
	})

	t.Run("Native", func(t *testing.T) {
		t.Parallel()

		t.Run("runs tsc over LSP on stdio", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Native().Command, []string{"tsc", "--lsp", "--stdio"}, "the command of tsc")
		})

		t.Run("opens a file as javascript", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Native().LanguageID, lsp.IdentityJavaScript, "the language identifier of tsc")
		})

		t.Run("passes Server.Valid", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, javascript.Native().Valid(), "Valid of tsc")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("returns tsc for a workspace on TypeScript 7", func(t *testing.T) {
			t.Parallel()
			files := fstest.MapFS{"package.json": {Data: []byte(`{"devDependencies": {"typescript": "^7.0.2"}}`)}}
			assert.Equal(t, javascript.For(files).Name, "tsc", "the server of the workspace")
		})

		t.Run("returns typescript-language-server for a workspace without TypeScript 7", func(t *testing.T) {
			t.Parallel()
			files := fstest.MapFS{"package.json": {Data: []byte(`{"devDependencies": {"typescript": "^6.0.3"}}`)}}
			assert.Equal(t, javascript.For(files).Name, "typescript-language-server", "the server of the workspace")
		})
	})
}

// BenchmarkGrammar measures the tree-sitter engine of JavaScript over one
// file of 48,000 declarations, the size of a large bundle.
func BenchmarkGrammar(b *testing.B) {
	const lines = 12_000
	e, err := treesitter.New(fstest.MapFS{"src/bundle.js": {Data: bundle(lines)}},
		javascript.Declaration(), javascript.Grammar())
	assert.NoError(b, err, "New over the bundle")
	b.Cleanup(e.Close)

	b.Run("Outline", func(b *testing.B) {
		got, err := e.Outline(b.Context(), engine.Request{Scope: engine.Root})
		assert.NoError(b, err, "Outline of the bundle")
		assert.Length(b, got.Items, 4*lines, "the declarations of the bundle")
		for b.Loop() {
			_, _ = e.Outline(b.Context(), engine.Request{Scope: engine.Root})
		}
	})

	b.Run("Search", func(b *testing.B) {
		q := engine.Query{Text: fmt.Sprintf("f%d", lines/2), Private: true}
		got, err := e.Search(b.Context(), engine.Request{Scope: engine.Root}, q)
		assert.NoError(b, err, "Search of the bundle")
		assert.Length(b, got.Items, 1, "the matches of "+q.Text)
		for b.Loop() {
			_, _ = e.Search(b.Context(), engine.Request{Scope: engine.Root}, q)
		}
	})
}

// bundle returns lines functions, each with two parameters and a constant,
// which the query of the module reads as four declarations.
func bundle(lines int) []byte {
	var out bytes.Buffer
	for i := range lines {
		fmt.Fprintf(&out, "function f%d(a, b) { const x%d = a + b; return x%d; }\n", i, i, i)
	}
	return out.Bytes()
}
