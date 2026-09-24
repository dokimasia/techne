// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		for _, shape := range []struct {
			mode lsptest.Mode
			name string
		}{
			{lsptest.Default, "a list of locations"},
			{lsptest.OneLocation, "one location"},
			{lsptest.Links, "a list of links"},
		} {
			t.Run("reads a definition sent as "+shape.name, func(t *testing.T) {
				t.Parallel()
				got, err := serving(t, shape.mode, sample()).
					Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
				assert.NoError(t, err, "Resolve of Store")
				assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
				assert.Equal(t, got.Items[0].Kind, sema.KindStruct, "the kind of Store")
			})
		}

		t.Run("reads the declaration at a definition through the outline engine", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			e := lsptest.Parsing(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Default, lsptest.RecordRequests(log)))
			got, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve of Store")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
			assert.Equal(t, requested(t, log, "textDocument/documentSymbol"), 0, "the requests for symbols")
		})

		t.Run("returns nothing for a name without a definition", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Unresolved, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve of a name without a definition")
			assert.Empty(t, got.Items, "the declarations of the name")
			assert.False(t, got.Skipped, "Skipped of the answer")
		})

		t.Run("returns a partial answer for a name without a definition", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Unresolved, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve of a name without a definition")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the empty answer")
		})

		t.Run("skips a scope of another language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"}).
				Resolve(t.Context(), engine.Request{Scope: "notes.md"}, store())
			assert.NoError(t, err, "Resolve in notes.md")
			assert.True(t, got.Skipped, "Skipped of the answer")
		})

		t.Run("skips a directory without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"}).
				Resolve(t.Context(), engine.Request{Scope: "."}, store())
			assert.NoError(t, err, "Resolve in the workspace root")
			assert.True(t, got.Skipped, "Skipped of the answer")
		})

		t.Run("declines a directory with files of the language", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).
				Resolve(t.Context(), engine.Request{Scope: "."}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve in a directory")
		})

		t.Run("declines a server without definitions", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Thin, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
			assert.Contains(t, err.Error(), "textDocument/definition", "the error of Resolve")
		})
	})
}
