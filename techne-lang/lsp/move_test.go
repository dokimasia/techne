// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// moved plans a move of the file from to the path to with e.
func moved(t *testing.T, e *lsp.Engine, from, to string) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: source.Path(from)}, edit.MoveFile,
		edit.Target{Kind: edit.TargetFile, Path: source.Path(from)},
		edit.Args{edit.ArgDestination: to})
}

// preloading returns an engine of a scoped server over swap/recipe.fake, which declares
// [lsptest.Content], and one file more than [Engine.preload] opens in another directory, each
// with the content text.
func preloading(t *testing.T, text string) *lsp.Engine {
	t.Helper()
	files := map[string]string{"swap/recipe.fake": lsptest.Content}
	for i := range 201 {
		files[fmt.Sprintf("other/f%d.fake", i)] = text
	}
	server := lsptest.Server(lsptest.Scoped)
	server.Scoped = true
	return lsptest.Engine(t, lsptest.Workspace(t, files), server)
}

func TestMove(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the edits of the server before the move", func(t *testing.T) {
			t.Parallel()
			got, err := moved(t, serving(t, lsptest.Default, sample()), "a.fake", "b.fake")
			assert.NoError(t, err, "Plan of a move")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit, edit.ChangeMove},
				"the changes of the plan")
			assert.Equal(t, got.Items[1].To, source.Path("b.fake"), "the destination of the move")
		})

		t.Run("opens the files that write the stem for a scoped server", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Scoped)
			server.Scoped = true
			root := lsptest.Workspace(t, map[string]string{
				"a.fake": lsptest.Content, "b.fake": "// imports a\nvar _ Store\n",
			})
			got, err := moved(t, lsptest.Engine(t, root, server), "a.fake", "c.fake")
			assert.NoError(t, err, "Plan of a move by a scoped server")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake", "b.fake", "a.fake"}, "the files the move changes")
		})

		t.Run("leaves out the files that write the stem in another directory for a scoped server", func(t *testing.T) {
			t.Parallel()
			got, err := moved(t, preloading(t, "var recipe = 1\n"), "swap/recipe.fake", "swap/motion.fake")
			assert.NoError(t, err, "Plan of a move by a scoped server")
			assert.False(t, cutShort(got.Caveats), "the plan has the caveat of a short preload")
		})

		t.Run("counts the files that write the directory and the stem for a scoped server", func(t *testing.T) {
			t.Parallel()
			got, err := moved(t, preloading(t, "// imports swap/recipe\n"), "swap/recipe.fake", "swap/motion.fake")
			assert.NoError(t, err, "Plan of a move by a scoped server")
			assert.True(t, cutShort(got.Caveats), "the plan has the caveat of a short preload")
		})

		t.Run("declines a server without willRenameFiles", func(t *testing.T) {
			t.Parallel()
			_, err := moved(t, serving(t, lsptest.Moveless, sample()), "a.fake", "b.fake")
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Plan")
			assert.Contains(t, err.Error(), "willRenameFiles", "the error of Plan")
		})

		t.Run("refuses a destination that exists", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, map[string]string{"a.fake": lsptest.Content, "b.fake": lsptest.Content})
			_, err := moved(t, e, "a.fake", "b.fake")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "overwrite", "the error of Plan")
		})

		t.Run("refuses a file that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := moved(t, serving(t, lsptest.Default, sample()), "gone.fake", "b.fake")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("refuses a destination outside the workspace", func(t *testing.T) {
			t.Parallel()
			_, err := moved(t, serving(t, lsptest.Default, sample()), "a.fake", "../b.fake")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "outside", "the error of Plan")
		})

		t.Run("moves a file whose name starts with two dots", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default,
				map[string]string{"a.fake": lsptest.Content, "..config.fake": lsptest.Content})
			got, err := moved(t, e, "..config.fake", "..settings.fake")
			assert.NoError(t, err, "Plan of a move of ..config.fake")
			assert.Equal(t, got.Items[len(got.Items)-1].To, source.Path("..settings.fake"), "the destination")
		})

		t.Run("refuses a move to the same path", func(t *testing.T) {
			t.Parallel()
			_, err := moved(t, serving(t, lsptest.Default, sample()), "a.fake", "a.fake")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("refuses a move without a destination", func(t *testing.T) {
			t.Parallel()
			_, err := moved(t, serving(t, lsptest.Default, sample()), "a.fake", "")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("refuses a target that is not a file", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Plan(t.Context(), engine.Request{}, edit.MoveFile,
				edit.Target{Kind: edit.TargetSymbol, Symbol: "fake::Store"},
				edit.Args{edit.ArgDestination: "b.fake"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("skips a file of another language", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"})
			got, err := moved(t, e, "notes.md", "docs.md")
			assert.NoError(t, err, "Plan of a move of notes.md")
			assert.True(t, got.Skipped, "Skipped of the plan")
		})
	})
}
