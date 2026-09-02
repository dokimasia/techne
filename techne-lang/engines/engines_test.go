// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engines_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// A grammar belongs to one language and lives in that language's module,
// so this one cannot build a parser and cannot reach the paths that need
// one. What it settles is the decision this package exists to make —
// which server a workspace supports — and every way building refuses.
// That a real parser and a real server register together is pinned by
// each language module, over its own grammar.

// declared is a language with one extension and nothing surprising about
// it.
func declared() lang.Declaration {
	return lang.Declaration{
		Language:   "fake",
		Extensions: []string{".fake"},
		Comment:    lang.CommentStyle{Line: "// "},
		IsTest:     func(p string) bool { return strings.HasSuffix(p, "_test.fake") },
		Namespace:  filepath.Dir,
		Visibility: func(string) sema.Visibility { return sema.Exported },
	}
}

// served is a declaration for a server that is not on this machine,
// which is the case every language module has to work in.
func served() lsp.Server {
	return lsp.Server{
		Name:       "fake-server",
		Command:    []string{"techne-no-such-language-server"},
		LanguageID: lsp.IdentityGo,
		Serves:     lsp.Binding(),
	}
}

// onDisk is a workspace a process can open files in.
func onDisk(t *testing.T) lang.Workspace {
	t.Helper()
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.fake"), []byte("{}"), 0o644),
		"the case can prepare the workspace")
	return lang.Workspace{FS: os.DirFS(dir), Root: dir}
}

// nowhere is a workspace that was never written to disk.
func nowhere() lang.Workspace { return lang.Workspace{FS: fstest.MapFS{}} }

func TestServing(t *testing.T) {
	t.Parallel()

	t.Run("Serving", func(t *testing.T) {
		t.Parallel()

		t.Run("builds a server for a workspace on disk", func(t *testing.T) {
			t.Parallel()
			built, has, err := engines.Serving(onDisk(t), declared(), served())

			assert.NoError(t, err, "a declared server builds over a directory")
			assert.True(t, has, "and the workspace supports one")
			assert.Equal(t, built.Name(), "fake-server", "named as the module declared it")
			assert.Equal(t, string(built.Language()), "fake", "answering about that language")
		})

		t.Run("builds one that is not installed", func(t *testing.T) {
			t.Parallel()
			// Told nothing, a caller concludes the language cannot be
			// served at all. Told the server is missing, it knows what to
			// install, which is a different problem and a fixable one.
			built, has, err := engines.Serving(onDisk(t), declared(), served())
			assert.NoError(t, err, "a missing server is still declared")
			assert.True(t, has, "and still registered")

			gate, reports := built.(engine.Available)
			assert.True(t, reports, "it reports on itself")
			assert.HasError(t, gate.Available(t.Context()), "saying it cannot run")
		})

		t.Run("builds none where nothing can open a file by name", func(t *testing.T) {
			t.Parallel()
			// A server pointed at a tree that was never written opens
			// nothing and is answered about nothing, and being answered
			// about nothing is what this project must never report as an
			// answer.
			_, has, err := engines.Serving(nowhere(), declared(), served())

			assert.NoError(t, err, "a workspace that is nowhere is not a fault")
			assert.False(t, has, "it simply supports no server")
		})

		t.Run("builds none for a language that declared none", func(t *testing.T) {
			t.Parallel()
			_, has, err := engines.Serving(onDisk(t), declared(), lsp.Server{})

			assert.NoError(t, err, "a language with no server declaration is not a fault")
			assert.False(t, has, "and gets no server engine")
		})

		t.Run("refuses a declaration a server cannot be run from", func(t *testing.T) {
			t.Parallel()
			// Caught where a module registers rather than on the first
			// call: a document opened under an empty identity is answered
			// about nothing, and the answer reads like an empty file.
			broken := served()
			broken.LanguageID = ""

			_, _, err := engines.Serving(onDisk(t), declared(), broken)
			assert.HasError(t, err, "an incomplete declaration is refused at registration")
			assert.Contains(t, err.Error(), "language id", "and the reason names what is missing")
		})
	})
}

func TestFor(t *testing.T) {
	t.Parallel()

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration the parser cannot serve", func(t *testing.T) {
			t.Parallel()
			// Nothing is registered when any part fails, so a language
			// cannot end up in a catalogue with half its engines.
			broken := declared()
			broken.Extensions = nil

			_, err := engines.For(onDisk(t), broken, treesitter.Grammar{}, served())
			assert.HasError(t, err, "a declaration with no extension routes no path")
		})

		t.Run("refuses a language with no grammar", func(t *testing.T) {
			t.Parallel()
			_, err := engines.For(nowhere(), declared(), treesitter.Grammar{}, lsp.Server{})
			assert.HasError(t, err, "a parser is what every workspace gets and needs a grammar")
		})
	})
}

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("registers nothing when building refuses", func(t *testing.T) {
			t.Parallel()
			// A rejected module must leave no engines behind, or a
			// catalogue holds one for a language nothing routes to.
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.HasError(t,
				engines.Register(onDisk(t), r, c, declared(), treesitter.Grammar{}, served()),
				"a language with no grammar has no parser and is refused")

			assert.Empty(t, r.Languages(), "the registry is untouched")
			assert.Empty(t, c.Capabilities(t.Context()), "as is the catalogue")
		})
	})
}
