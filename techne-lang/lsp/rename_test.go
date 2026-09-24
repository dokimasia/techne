// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestRename(t *testing.T) {
	t.Parallel()

	both := func() map[string]string {
		return map[string]string{"a.fake": lsptest.Content, "b.fake": "var _ Store\n"}
	}

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("opens the files of the uses before the rename", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Opened, both()))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake", "b.fake"}, "the files the rename changes")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
		})

		t.Run("opens the files that write the name for a scoped server", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Scoped)
			server.Scoped = true
			got, err := renameWith(t, lsptest.Engine(t, lsptest.Workspace(t, both()), server))
			assert.NoError(t, err, "Plan of a rename by a scoped server")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake", "b.fake"}, "the files the rename changes")
		})

		t.Run("returns a partial plan that leaves a use unrewritten", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Short, both()))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatUnrewritten), "the plan has an unrewritten caveat")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the plan has a warming caveat")
		})

		t.Run("refuses a position the server cannot rename", func(t *testing.T) {
			t.Parallel()
			_, err := renameWith(t, serving(t, lsptest.Unnameable, sample()))
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("refuses a rename that the server refuses", func(t *testing.T) {
			t.Parallel()
			_, err := renameWith(t, serving(t, lsptest.Conflicts, sample()))
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "conflicts", "the error of Plan")
		})

		t.Run("refuses a rename without a new name", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
				edit.Args{})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("plans from a span in a declaration", func(t *testing.T) {
			t.Parallel()
			got, err := spanned(t, serving(t, lsptest.Default, sample()), source.Position{Offset: 11})
			assert.NoError(t, err, "Plan from byte 11")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake"}, "the files the rename changes")
		})

		t.Run("refuses a declaration that no file declares", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Missing", sema.KindStruct)},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("skips a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"}).Plan(t.Context(),
				engine.Request{Scope: "."}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan in a scope without a file of the language")
			assert.True(t, got.Skipped, "Skipped of the plan")
		})
	})
}
