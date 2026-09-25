// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"slices"
	"strings"
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
			assert.True(t, unrewritten(got.Caveats, "b.fake:1 that the rename does not rewrite"),
				"the plan has an unrewritten caveat that names b.fake:1")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the plan has a warming caveat")
		})

		t.Run("returns a partial plan for a use in a file that .gitignore excludes", func(t *testing.T) {
			t.Parallel()
			files := both()
			files[".gitignore"] = "b.fake\n"
			got, err := renameWith(t, serving(t, lsptest.Opened, files))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake"}, "the files the rename changes")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
			assert.True(t, unrewritten(got.Caveats, "b.fake:1 in a file that techne does not read"),
				"the plan has an unrewritten caveat that names b.fake:1")
		})

		t.Run("returns a partial plan that moves a file for a server without willRenameFiles", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Moveless, sample(), lsptest.Renames(moving())))
			assert.NoError(t, err, "Plan of a rename that moves a.fake")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
			assert.True(t, unrewritten(got.Caveats, "moves a.fake to vault.fake"),
				"the plan has an unrewritten caveat that names the move")
		})

		t.Run("returns a total plan that moves a file for a server with willRenameFiles", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Default, sample(), lsptest.Renames(moving())))
			assert.NoError(t, err, "Plan of a rename that moves a.fake")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
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

// The edits of the uses of Store in [lsptest.Content] in Get and in After that a rename to Vault
// writes, as the protocol writes them.
const (
	vaultInGet   = `{"range":{"start":{"line":6,"character":9},"end":{"line":6,"character":14}},"newText":"Vault"}`
	vaultInAfter = `{"range":{"start":{"line":8,"character":28},"end":{"line":8,"character":33}},"newText":"Vault"}`
)

// moving returns the workspace edit of a rename of Store to Vault that rewrites the declaration
// and both uses in a.fake, and then moves a.fake to vault.fake.
func moving() string {
	return ordered(documentEdit("{file}", vault, vaultInGet, vaultInAfter),
		`{"kind":"rename","oldUri":"{file}","newUri":"{root}/vault.fake"}`)
}

// unrewritten reports whether caveats contain a [trust.CaveatUnrewritten] caveat whose note
// contains text.
func unrewritten(caveats []trust.Caveat, text string) bool {
	return slices.ContainsFunc(caveats, func(one trust.Caveat) bool {
		return one.Code == trust.CaveatUnrewritten && strings.Contains(one.Note, text)
	})
}
