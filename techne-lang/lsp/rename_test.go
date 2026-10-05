// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"fmt"
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

		t.Run("returns a total plan that renames each use by an insertion at its end", func(t *testing.T) {
			t.Parallel()
			inserting := `{"changes":{"{file}":[` +
				`{"range":{"start":{"line":2,"character":10},"end":{"line":2,"character":10}},"newText":"X"},` +
				`{"range":{"start":{"line":6,"character":14},"end":{"line":6,"character":14}},"newText":"X"},` +
				`{"range":{"start":{"line":8,"character":33},"end":{"line":8,"character":33}},"newText":"X"}]}}`
			got, err := renameWith(t, serving(t, lsptest.Default, sample(), lsptest.Renames(inserting)))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
		})

		t.Run("returns a total plan that renames each use by an insertion at its start", func(t *testing.T) {
			t.Parallel()
			inserting := `{"changes":{"{file}":[` +
				`{"range":{"start":{"line":2,"character":5},"end":{"line":2,"character":5}},"newText":"X"},` +
				`{"range":{"start":{"line":6,"character":9},"end":{"line":6,"character":9}},"newText":"X"},` +
				`{"range":{"start":{"line":8,"character":28},"end":{"line":8,"character":28}},"newText":"X"}]}}`
			got, err := renameWith(t, serving(t, lsptest.Default, sample(), lsptest.Renames(inserting)))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
		})

		t.Run("returns a total plan that leaves a use without the name as it is", func(t *testing.T) {
			t.Parallel()
			// The server names the Other of line 8 as a use of Store, as csharp-ls names the new of
			// a target-typed new().
			files := map[string]string{"a.fake": strings.Replace(lsptest.Content, "(&Store{})", "(&Other{})", 1)}
			renaming := `{"changes":{"{file}":[` +
				`{"range":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}},"newText":"Vault"},` +
				`{"range":{"start":{"line":6,"character":9},"end":{"line":6,"character":14}},"newText":"Vault"}]}}`
			got, err := renameWith(t, serving(t, lsptest.Default, files, lsptest.Renames(renaming)))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
		})

		t.Run("returns a partial plan that inserts at no use", func(t *testing.T) {
			t.Parallel()
			inserting := `{"changes":{"{file}":[` +
				`{"range":{"start":{"line":2,"character":10},"end":{"line":2,"character":10}},"newText":"X"},` +
				`{"range":{"start":{"line":6,"character":14},"end":{"line":6,"character":14}},"newText":"X"}]}}`
			got, err := renameWith(t, serving(t, lsptest.Default, sample(), lsptest.Renames(inserting)))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
		})

		t.Run("returns a total plan that renames the name of a use over its qualifier", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"a.fake": lsptest.Content, "b.fake": "var _ a.Store\n"}
			got, err := renameWith(t, serving(t, lsptest.Qualified, files))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake", "b.fake"}, "the files the rename changes")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
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

		t.Run("renames from a use when the server prepares no rename at the declaration", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.FromUse, sample()))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake"}, "the files the rename changes")
		})

		// The rename of the scripted server leaves the use on line 9, which only the references
		// from the use on line 7 name.
		t.Run("returns a partial plan of a rename from a use that leaves a use from there", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.FromUse, sample()))
			assert.NoError(t, err, "Plan of a rename")
			assert.True(t, unrewritten(got.Caveats, "a.fake:9 that the rename does not rewrite"),
				"the plan has an unrewritten caveat that names a.fake:9")
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

		t.Run("aims a span at a local that the server does not list", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Locals})
			e := lsptest.Parsing(t, root, lsptest.Server(lsptest.Aims))
			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				parsed(t, root, "t"), edit.Args{edit.ArgNewName: "count"})
			assert.NoError(t, err, "Plan of the rename of the local t")
			assert.Length(t, got.Items, 1, "the changes of the plan")
			assert.Equal(t, got.Items[0].Edits[0].Span.Start.Offset, strings.Index(lsptest.Locals, "var t")+len("var "),
				"the offset of the edit")
		})

		t.Run("aims a span at the name of its ID among the names of one declaration", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Pair})
			e := lsptest.Parsing(t, root, lsptest.Server(lsptest.Aims))
			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				parsed(t, root, "t"), edit.Args{edit.ArgNewName: "count"})
			assert.NoError(t, err, "Plan of the rename of t")
			assert.Length(t, got.Items, 1, "the changes of the plan")
			assert.Equal(t, got.Items[0].Edits[0].Span.Start.Offset, strings.Index(lsptest.Pair, "s, t")+len("s, "),
				"the offset of the edit")
		})

		t.Run("aims an ID at its own name among the names of one declaration that the server lists",
			func(t *testing.T) {
				t.Parallel()
				got, err := serving(t, lsptest.Aims, map[string]string{"a.fake": lsptest.Pair}).Plan(t.Context(),
					engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
					edit.Target{Kind: edit.TargetSymbol, Symbol: declared("s", sema.KindVariable)},
					edit.Args{edit.ArgNewName: "count"})
				assert.NoError(t, err, "Plan of the rename of s")
				assert.Length(t, got.Items, 1, "the changes of the plan")
				assert.Equal(t, got.Items[0].Edits[0].Span.Start.Offset, strings.Index(lsptest.Pair, "s, t"),
					"the offset of the edit")
			})

		t.Run("opens no other file for a rename of a local by a scoped server", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"a.fake": lsptest.Locals}
			for i := range 201 {
				files[fmt.Sprintf("other/f%d.fake", i)] = "var t = 1\n"
			}
			root := lsptest.Workspace(t, files)
			server := lsptest.Server(lsptest.Aims)
			server.Scoped = true
			got, err := lsptest.Parsing(t, root, server).Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol, parsed(t, root, "t"), edit.Args{edit.ArgNewName: "count"})
			assert.NoError(t, err, "Plan of the rename of the local t")
			assert.False(t, cutShort(got.Caveats), "the plan has the caveat of a short preload")
		})

		t.Run("opens no file outside a project of tsserver that cannot refer to the declaration", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, projectless(t, map[string]string{"a.fake": moduleContent}, useOfStore, nil))
			assert.NoError(t, err, "Plan of a rename")
			assert.False(t, cutShort(got.Caveats), "the plan has the caveat of a short preload")
		})

		t.Run("opens every file outside a project of tsserver for a position that names no declaration",
			func(t *testing.T) {
				t.Parallel()
				e := projectless(t, map[string]string{"a.fake": moduleContent + "Store\n"}, useOfStore, nil)
				got, err := spanned(t, e, source.Position{Offset: len(moduleContent), Line: 10})
				assert.NoError(t, err, "Plan of a rename at line 11")
				assert.True(t, cutShort(got.Caveats), "the plan has the caveat of a short preload")
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

// parsed returns the target by which a tool addresses the declaration name that [lsptest.Parser]
// reads in a.fake of the workspace at root: the span of the declaration with its ID.
func parsed(t *testing.T, root, name string) edit.Target {
	t.Helper()
	got, err := lsptest.Parser(root).Outline(t.Context(), engine.Request{Scope: "a.fake"})
	assert.NoError(t, err, "Outline of a.fake")
	for _, one := range got.Items {
		if one.Name == name {
			return edit.Target{Kind: edit.TargetSpan, Symbol: one.ID, Span: one.Span}
		}
	}
	t.Fatalf("a.fake declares no %s", name)
	return edit.Target{}
}

// unrewritten reports whether caveats contain a [trust.CaveatUnrewritten] caveat whose note
// contains text.
func unrewritten(caveats []trust.Caveat, text string) bool {
	return slices.ContainsFunc(caveats, func(one trust.Caveat) bool {
		return one.Code == trust.CaveatUnrewritten && strings.Contains(one.Note, text)
	})
}
