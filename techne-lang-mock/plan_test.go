// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/mock"
)

// planned returns the plan of op over [workspace] for a request of src, and fails the test when
// Plan returns an error.
func planned(t *testing.T, op edit.Operation, target edit.Target, args edit.Args) engine.Result[edit.Change] {
	t.Helper()
	got, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, op, target, args)
	assert.NoError(t, err, "Plan of "+string(op))
	return got
}

// pointing returns the target of the declaration of [workspace] named name, by its span.
func pointing(t *testing.T, name string) edit.Target {
	t.Helper()
	got, err := built(t).Outline(t.Context(), engine.Request{Scope: engine.Root})
	assert.NoError(t, err, "Outline of the workspace")
	return edit.Target{Kind: edit.TargetSpan, Span: declared(t, got.Items, name).Span}
}

// applied returns the content of each file that changes edit, as the write path writes it.
func applied(t *testing.T, fsys fstest.MapFS, changes []edit.Change) map[source.Path]string {
	t.Helper()
	out := map[source.Path]string{}
	for _, one := range changes {
		content, err := edit.Apply(fsys[string(one.Path)].Data, one.Edits)
		assert.NoError(t, err, "Apply to "+string(one.Path))
		out[one.Path] = string(content)
	}
	return out
}

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("renames a declaration with every use of it in every file", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Vault"})
			assert.Equal(t, applied(t, workspace(), got.Items), map[source.Path]string{
				"src/client.mock": "func Client\n  use New\n  use Vault\n",
				"src/store.mock": ";; Store maps a name to an item.\ntype Vault\n  field size\n  method Get\n" +
					"    use Vault\n\nfunc New\n  use Vault\n",
			}, "the files after the rename")
		})

		t.Run("renames the uses in every file for a request of one file", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Plan(t.Context(), engine.Request{Scope: "src/store.mock"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of the rename")
			assert.Equal(t, applied(t, workspace(), got.Items)["src/client.mock"],
				"func Client\n  use New\n  use Vault\n", "src/client.mock after the rename")
		})

		t.Run("renames the declaration that an ID names", func(t *testing.T) {
			t.Parallel()
			target := edit.Target{Kind: edit.TargetSymbol, Symbol: id("New", sema.KindFunction)}
			got := planned(t, edit.RenameSymbol, target, edit.Args{edit.ArgNewName: "Make"})
			assert.Equal(t, applied(t, workspace(), got.Items), map[source.Path]string{
				"src/client.mock": "func Client\n  use Make\n  use Store\n",
				"src/store.mock": ";; Store maps a name to an item.\ntype Store\n  field size\n  method Get\n" +
					"    use Store\n\nfunc Make\n  use Store\n",
			}, "the files after the rename")
		})

		t.Run("refuses the name that the declaration has", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Store"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename to Store")
		})

		t.Run("refuses an empty name", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "  "})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename to a blank name")
		})

		t.Run("refuses a name of two words", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Two Words"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename to two words")
		})

		t.Run("refuses a declaration whose name another declaration shares", func(t *testing.T) {
			t.Parallel()
			_, err := over(t, twins()).Plan(t.Context(), engine.Request{Scope: "src"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: id("Store.Get", sema.KindMethod)},
				edit.Args{edit.ArgNewName: "Fetch"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename of Store.Get")
		})

		t.Run("refuses a span that starts at no declaration", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "src/store.mock"}},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename at the documentation")
		})

		t.Run("refuses an ID that no declaration has", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: id("Absent", sema.KindType)},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename of Absent")
			assert.ErrorIsNot(t, err, engine.ErrDecline, "the error of the rename of Absent")
		})

		t.Run("refuses an ID that two declarations have", func(t *testing.T) {
			t.Parallel()
			_, err := over(t, doubles()).Plan(t.Context(), engine.Request{Scope: "src"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: id("Twice", sema.KindFunction)},
				edit.Args{edit.ArgNewName: "Once"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the rename of Twice")
			assert.Contains(t, err.Error(), "names 2 declarations", "the reason of the refusal")
		})

		t.Run("writes the documentation above a declaration without one", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.DocumentSymbol, pointing(t, "New"), edit.Args{edit.ArgDoc: "Make one."})
			assert.Equal(t, applied(t, workspace(), got.Items)["src/store.mock"],
				";; Store maps a name to an item.\ntype Store\n  field size\n  method Get\n    use Store\n\n"+
					";; Make one.\nfunc New\n  use Store\n", "src/store.mock after the documentation")
		})

		t.Run("replaces the documentation above a declaration", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.DocumentSymbol, pointing(t, "Store"), edit.Args{edit.ArgDoc: "Something else."})
			assert.Equal(t, applied(t, workspace(), got.Items)["src/store.mock"],
				";; Something else.\ntype Store\n  field size\n  method Get\n    use Store\n\nfunc New\n  use Store\n",
				"src/store.mock after the documentation")
		})

		t.Run("indents the documentation of a nested declaration", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.DocumentSymbol, pointing(t, "Get"), edit.Args{edit.ArgDoc: "Returns one."})
			assert.Equal(t, applied(t, workspace(), got.Items)["src/store.mock"],
				";; Store maps a name to an item.\ntype Store\n  field size\n  ;; Returns one.\n  method Get\n"+
					"    use Store\n\nfunc New\n  use Store\n", "src/store.mock after the documentation")
		})

		t.Run("writes a line of documentation per line of the text", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.DocumentSymbol, pointing(t, "Client"), edit.Args{edit.ArgDoc: "One.\nTwo."})
			assert.Equal(t, applied(t, workspace(), got.Items)["src/client.mock"],
				";; One.\n;; Two.\nfunc Client\n  use New\n  use Store\n", "src/client.mock after the documentation")
		})

		t.Run("ends each line of documentation with the line ending of the file", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"a.mock": {Data: []byte("type Store\r\n")}}
			got, err := over(t, fsys).Plan(t.Context(), engine.Request{Scope: "a.mock"}, edit.DocumentSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: sema.NewID(mock.Language, "", "Store", sema.KindType)},
				edit.Args{edit.ArgDoc: "One."})
			assert.NoError(t, err, "Plan of the documentation")
			assert.Equal(t, applied(t, fsys, got.Items)["a.mock"], ";; One.\r\ntype Store\r\n",
				"a.mock after the documentation")
		})

		t.Run("refuses a blank documentation", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.DocumentSymbol, pointing(t, "New"), edit.Args{edit.ArgDoc: "  "})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of a blank documentation")
		})

		t.Run("moves a file of the language", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.MoveFile, edit.Target{Kind: edit.TargetFile, Path: "src/client.mock"},
				edit.Args{edit.ArgDestination: "lib/client.mock"})
			want := []edit.Change{{Kind: edit.ChangeMove, Path: "src/client.mock", To: "lib/client.mock"}}
			assert.Equal(t, got.Items, want, "the changes of the move")
		})

		t.Run("refuses a move of a file that the workspace does not contain", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, edit.MoveFile,
				edit.Target{Kind: edit.TargetFile, Path: "src/absent.mock"}, edit.Args{edit.ArgDestination: "b.mock"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the move of src/absent.mock")
		})

		t.Run("refuses a move of a directory", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, edit.MoveFile,
				edit.Target{Kind: edit.TargetFile, Path: "src"}, edit.Args{edit.ArgDestination: "lib"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the move of src")
		})

		t.Run("refuses a move without a destination", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, edit.MoveFile,
				edit.Target{Kind: edit.TargetFile, Path: "src/client.mock"}, edit.Args{})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of the move without a destination")
		})

		t.Run("returns a skipped result for a file of another language", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.MoveFile, edit.Target{Kind: edit.TargetFile, Path: "notes.md"},
				edit.Args{edit.ArgDestination: "docs/notes.md"})
			assert.True(t, got.Skipped, "the skip of the move of notes.md")
			assert.Empty(t, got.Items, "the changes of the move of notes.md")
		})

		t.Run("declines an operation without a planner", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.ExtractFunction, pointing(t, "New"), edit.Args{edit.ArgNewName: "x"})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of the extraction")
		})

		t.Run("returns the completeness that Covering sets", func(t *testing.T) {
			t.Parallel()
			got, err := built(t, mock.Covering(trust.ScopePartial)).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of the rename")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
		})

		t.Run("returns a partial plan for a workspace with a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			fsys := workspace()
			fsys["big.mock"] = &fstest.MapFile{Data: bytes.Repeat([]byte("x"), lang.Largest+1)}
			got, err := over(t, fsys).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of the rename")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
			assert.Equal(t, codes(got.Caveats), []trust.CaveatCode{trust.CaveatDynamic, trust.CaveatUnread},
				"the caveats of the plan")
		})
	})
}
