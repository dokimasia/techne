// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// projects returns an engine over two projects, one and two, each marked by an x.manifest
// file, whose scripted server publishes an error for every file of two.
func projects(t *testing.T) *lsp.Engine {
	t.Helper()
	root := lsptest.Workspace(t, map[string]string{
		"one/x.manifest": "", "one/a.fake": lsptest.Content,
		"two/x.manifest": "", "two/b.fake": lsptest.Content,
	})
	d := lsptest.Declaration()
	d.Manifests = []string{"x.manifest"}
	e, err := lsp.New(root, d, lsptest.Server(lsptest.PushesOne), nil)
	assert.NoError(t, err, "New over two projects")
	lsptest.Cleanup(t, e)
	return e
}

// The lines that the tests append to [lsptest.Content] as line 9. The Compiles mode reports an
// error on each. The first writes Store, the second writes the stem of a.fake, and the third
// writes only its own variable.
const (
	namingStore = "var shadow " + lsptest.Broken + " // Store\n"
	namingStem  = "var shadow " + lsptest.Broken + " // a\n"
	naming      = "var shadow " + lsptest.Broken + "\n"
)

// pulled returns an engine over a.fake with content, whose Compiles server has reported the
// errors of the file.
func pulled(t *testing.T, mode lsptest.Mode, content string) *lsp.Engine {
	t.Helper()
	e := serving(t, mode, map[string]string{"a.fake": content})
	_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
	assert.NoError(t, err, "Verify pulls the report of a.fake")
	return e
}

func TestEvidence(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an answer about a use on a line with an error", func(t *testing.T) {
			t.Parallel()
			got, err := projects(t).Resolve(t.Context(), engine.Request{Scope: "two/b.fake"}, store())
			assert.NoError(t, err, "Resolve in the project with an error")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the answer")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatBuildBroken), "the answer has a build caveat")
		})

		t.Run("keeps the tier of an answer whose line has no error", func(t *testing.T) {
			t.Parallel()
			got, err := pulled(t, lsptest.Compiles, lsptest.Faulty).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve with an error on another line")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatBuildBroken), "the answer has a build caveat")
		})

		t.Run("lowers an empty answer about a file with an error", func(t *testing.T) {
			t.Parallel()
			got, err := pulled(t, lsptest.Unbound, lsptest.Faulty).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve of a name that the error leaves unbound")
			assert.Empty(t, got.Items, "the declarations of an unbound name")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the empty answer")
		})

		t.Run("keeps the tier of a project beside a project with an error", func(t *testing.T) {
			t.Parallel()
			e := projects(t)
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "two/b.fake"}, store())
			assert.NoError(t, err, "Resolve in the project with an error")

			got, err := e.Resolve(t.Context(), engine.Request{Scope: "one/a.fake"}, store())
			assert.NoError(t, err, "Resolve in the project without an error")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatBuildBroken), "the answer has a build caveat")
		})

		t.Run("lowers an answer after a pulled report of an error at the use", func(t *testing.T) {
			t.Parallel()
			got, err := pulled(t, lsptest.Compiles, "package a\n\ntype Store "+lsptest.Broken+"\n").
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve after the pulled report")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the answer")
		})

		t.Run("returns the dynamic caveat", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatDynamic), "the answer has a dynamic caveat")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a partial empty answer from a server without a view of the file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Ungated, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.Implements)
			assert.NoError(t, err, "Relate with a server without a view")
			assert.Empty(t, got.Items, "the implementations from a server without a view")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
		})

		t.Run("returns a total empty answer from a server that published diagnostics", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Pushes, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.Implements)
			assert.NoError(t, err, "Relate with a server that publishes")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("lowers an answer when an error is on a line that writes the name", func(t *testing.T) {
			t.Parallel()
			got, err := pulled(t, lsptest.Compiles, lsptest.Content+namingStore).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with an error on a line that writes Store")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the answer")
		})

		t.Run("keeps the tier of an answer when the error is on the declaration", func(t *testing.T) {
			t.Parallel()
			got, err := projects(t).Relate(t.Context(),
				engine.Request{Scope: "two/b.fake"}, sema.NewID(lsptest.Language, "two", "Store", sema.KindStruct),
				sema.ReferencedBy)
			assert.NoError(t, err, "Relate with an error on the line of the declaration")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
		})

		t.Run("keeps the tier of an answer when no error line writes the name", func(t *testing.T) {
			t.Parallel()
			got, err := pulled(t, lsptest.Compiles, lsptest.Content+naming).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with an error on a line that does not write Store")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
		})
	})

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a partial move from a server that computes no edit", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.SilentMove, sample()).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.MoveFile,
				edit.Target{Kind: edit.TargetFile, Path: "a.fake"},
				edit.Args{edit.ArgDestination: "b.fake"})
			assert.NoError(t, err, "Plan of a move")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeMove}, "the changes of the plan")
		})

		renaming := func(t *testing.T, content string) engine.Result[edit.Change] {
			t.Helper()
			got, err := pulled(t, lsptest.Compiles, content).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of a rename")
			return got
		}

		t.Run("lowers a rename when an error is on a line that writes the old name", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, renaming(t, lsptest.Content+namingStore).Lowered, trust.Indexed,
				"the lowered tier of the rename")
		})

		t.Run("keeps the tier of a rename when no error line writes the old name", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, renaming(t, lsptest.Content+naming).Lowered, trust.None,
				"the lowered tier of the rename")
		})

		moving := func(t *testing.T, content string) engine.Result[edit.Change] {
			t.Helper()
			got, err := pulled(t, lsptest.Compiles, content).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.MoveFile,
				edit.Target{Kind: edit.TargetFile, Path: "a.fake"},
				edit.Args{edit.ArgDestination: "c.fake"})
			assert.NoError(t, err, "Plan of a move")
			return got
		}

		t.Run("lowers a move when an error is on a line that writes the stem", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, moving(t, lsptest.Content+namingStem).Lowered, trust.Indexed,
				"the lowered tier of the move")
		})

		t.Run("keeps the tier of a move when no error line writes the stem", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, moving(t, lsptest.Content+namingStore).Lowered, trust.None,
				"the lowered tier of the move")
		})
	})
}

// kinds returns the kind of each change, in order.
func kinds(changes []edit.Change) []edit.ChangeKind {
	out := make([]edit.ChangeKind, 0, len(changes))
	for _, one := range changes {
		out = append(out, one.Kind)
	}
	return out
}
