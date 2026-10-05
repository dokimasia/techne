// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/javascript"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// formatting is the section that the test pins: typescript-language-server requests the
// indentation of a file under it.
const formatting = "formattingOptions"

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

		t.Run("declares that the server returns the imported file at an import", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Server().Imports, "Imports of typescript-language-server")
		})

		t.Run("declares how long after its start the server resolves an import late", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().Resolving, lsp.TypeScriptResolving,
				"Resolving of typescript-language-server")
		})

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

		t.Run("declares the server quiet", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Server().Quiet, "Quiet of typescript-language-server")
		})

		t.Run("declares that tsserver returns the uses of redeclared members", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Server().Related, "Related of typescript-language-server")
		})

		t.Run("declares that tsserver returns expressions among implementations", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Server().Contextual, "Contextual of typescript-language-server")
		})

		t.Run("declares that the server forwards the requests of tsserver", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Server().Tsserver, "Tsserver of typescript-language-server")
		})

		t.Run("names the section under which the server requests the indentation of a file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().Indentation, lsp.Indentation{Options: formatting},
				"the indentation section of typescript-language-server")
		})

		t.Run("sends the settings under which a rename rewrites every use", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Server().Settings, lsp.TypeScriptSettings(),
				"the settings of typescript-language-server")
		})
	})

	t.Run("Native", func(t *testing.T) {
		t.Parallel()

		t.Run("declares that the server returns the imported file at an import", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.Native().Imports, "Imports of tsc")
		})

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

		t.Run("declares that tsc publishes a report after every change", func(t *testing.T) {
			t.Parallel()
			assert.False(t, javascript.Native().Quiet, "Quiet of tsc")
		})

		t.Run("sends the settings under which a rename rewrites every use", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Native().Settings, lsp.NativeSettings(), "the settings of tsc")
		})
	})

	t.Run("Grammar", func(t *testing.T) {
		t.Parallel()

		// A file of declarations at its top level without a bracket before them makes each
		// declaration read the text of the file before it, when the reading is not bounded. The
		// time of the outline then grows with the square of the declarations: 16 times the time
		// for 4 times the declarations, against 4 times when the reading is bounded.
		t.Run("outlines four times the declarations without brackets in less than eight times the time",
			func(t *testing.T) {
				t.Parallel()
				small, large := flat(t, 15_000), flat(t, 60_000)
				assert.True(
					t,
					large < 8*small,
					fmt.Sprintf("15,000 declarations took %s and 60,000 took %s", small, large),
				)
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

// flat returns the fastest of three outlines of a file of n declarations at its top level
// without a bracket before them, each by a new engine, which keeps no declaration of the file.
func flat(t *testing.T, n int) time.Duration {
	t.Helper()
	var body bytes.Buffer
	for i := range n {
		fmt.Fprintf(&body, "var a%d = 1;\n", i)
	}
	fastest := time.Duration(math.MaxInt64)
	for range 3 {
		e, err := treesitter.New(fstest.MapFS{"flat.js": {Data: body.Bytes()}},
			javascript.Declaration(), javascript.Grammar())
		assert.NoError(t, err, "New over the file")
		start := time.Now()
		got, err := e.Outline(t.Context(), engine.Request{Scope: "flat.js"})
		fastest = min(fastest, time.Since(start))
		e.Close()
		assert.NoError(t, err, "Outline of the file")
		assert.Length(t, got.Items, n, "the declarations of the file")
	}
	return fastest
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
