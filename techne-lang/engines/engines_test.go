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

// The grammar of a language is in the module of that language, so these tests build no
// tree-sitter engine. Each language module tests its engines over its own grammar.

// declared returns the declaration of a language fake with the extension .fake.
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

// served returns the declaration of a server whose program is not on PATH.
func served() lsp.Server {
	return lsp.Server{
		Name:       "fake-server",
		Command:    []string{"techne-no-such-language-server"},
		LanguageID: lsp.IdentityGo,
		Serves:     lsp.Binding(),
	}
}

// onDisk returns a workspace in a new temporary directory with the file a.fake.
func onDisk(t *testing.T) lang.Workspace {
	t.Helper()
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.fake"), []byte("{}"), 0o644),
		"the test writes a.fake")
	return lang.Workspace{FS: os.DirFS(dir), Root: dir}
}

// inMemory returns a workspace that is not on disk.
func inMemory() lang.Workspace { return lang.Workspace{FS: fstest.MapFS{}} }

func TestEngines(t *testing.T) {
	t.Parallel()

	t.Run("Serving", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the server engine of a workspace on disk", func(t *testing.T) {
			t.Parallel()
			built, has, err := engines.Serving(onDisk(t), declared(), served(), nil)
			assert.NoError(t, err, "the error of Serving")
			assert.True(t, has, "the report of a server engine")
			assert.Equal(t, built.Name(), "fake-server", "the name of the engine")
			assert.Equal(t, string(built.Language()), "fake", "the language of the engine")
		})

		t.Run("returns an engine for a server that is not on PATH", func(t *testing.T) {
			t.Parallel()
			built, has, err := engines.Serving(onDisk(t), declared(), served(), nil)
			assert.NoError(t, err, "the error of Serving")
			assert.True(t, has, "the report of a server engine")
			reported, available := built.(engine.Available)
			assert.True(t, available, "the engine implements engine.Available")
			err = reported.Available(t.Context())
			assert.HasError(t, err, "the error of Available")
			assert.Contains(t, err.Error(), "techne-no-such-language-server", "the error of Available")
		})

		t.Run("returns no engine for a workspace in memory", func(t *testing.T) {
			t.Parallel()
			_, has, err := engines.Serving(inMemory(), declared(), served(), nil)
			assert.NoError(t, err, "the error of Serving")
			assert.False(t, has, "the report of a server engine")
		})

		t.Run("returns no engine for a declaration without a server", func(t *testing.T) {
			t.Parallel()
			_, has, err := engines.Serving(onDisk(t), declared(), lsp.Server{}, nil)
			assert.NoError(t, err, "the error of Serving")
			assert.False(t, has, "the report of a server engine")
		})

		t.Run("refuses a server without a language identifier", func(t *testing.T) {
			t.Parallel()
			broken := served()
			broken.LanguageID = ""
			_, _, err := engines.Serving(onDisk(t), declared(), broken, nil)
			assert.HasError(t, err, "the error of Serving")
			assert.Contains(t, err.Error(), "language id", "the error of Serving")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration without an extension", func(t *testing.T) {
			t.Parallel()
			broken := declared()
			broken.Extensions = nil
			_, err := engines.For(onDisk(t), broken, treesitter.Grammar{}, served())
			assert.HasError(t, err, "the error of For")
		})

		t.Run("refuses a language without a grammar", func(t *testing.T) {
			t.Parallel()
			_, err := engines.For(inMemory(), declared(), treesitter.Grammar{}, lsp.Server{})
			assert.HasError(t, err, "the error of For")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("registers nothing when an engine fails to build", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			err := engines.Register(onDisk(t), r, c, declared(), treesitter.Grammar{}, served())
			assert.HasError(t, err, "the error of Register")
			assert.Empty(t, r.Languages(), "the languages of the registry")
			assert.Empty(t, c.Capabilities(t.Context()), "the capabilities of the catalogue")
		})
	})
}
