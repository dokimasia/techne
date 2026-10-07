// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
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

		t.Run("returns the declarations of a new server when the server has stopped answering", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Parsing(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Mutes, lsptest.RecordStarts(log)))
			got, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve of Store")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
			assert.Equal(t, starts(t, log), 2, "the starts of the server")
		})

		t.Run("returns ErrDecline when a new server has stopped answering too", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Parsing(t, lsptest.Workspace(t, sample()), lsptest.Server(lsptest.Mutes))
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
			assert.Contains(t, err.Error(), "fake returned no symbol of a.fake", "the error of Resolve")
		})

		t.Run("follows a definition that no declaration contains to its own definition", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Exports, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Line: 8, Column: 28})
			assert.NoError(t, err, "Resolve of a name whose definition is an export")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that the name denotes")
		})

		t.Run("asks again after the report of a file whose first definition is empty", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Loads, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve in a file that the server loads")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
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

		t.Run("refuses a directory with files of the language", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).
				Resolve(t.Context(), engine.Request{Scope: "."}, store())
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Resolve in a directory")
		})

		t.Run("refuses a line past the end of the file", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			_, err := serving(t, lsptest.Default, sample(), lsptest.RecordRequests(log)).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Line: 999})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Resolve past the end")
			assert.Equal(t, requested(t, log, "textDocument/definition"), 0, "the requests for a definition")
		})

		t.Run("refuses a column past the end of its line", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Line: 2, Column: 999})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Resolve past the end of a line")
		})

		t.Run("declines a server without definitions", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Thin, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
			assert.Contains(t, err.Error(), "textDocument/definition", "the error of Resolve")
		})

		t.Run("adds the waits for the server to the Waited of the context", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "the Resolve that starts the server")
			ctx, waited := engine.Timing(t.Context())
			_, err = e.Resolve(ctx, engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve")
			assert.InRange(t, waited.Total(), 1, 1<<63, "the time that Resolve waited for the server")
		})

		t.Run("declines a definition that the server cancels", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Cancels, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
			assert.Contains(t, err.Error(), "cancelled", "the error of Resolve")
		})
	})
}
