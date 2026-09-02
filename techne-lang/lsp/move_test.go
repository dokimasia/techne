// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
)

// moved asks for a file to be moved, which is how every case here begins.
func moved(
	t *testing.T,
	e *lsp.Engine,
	from, to string,
) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: source.Path(from)}, edit.MoveFile,
		edit.Target{Kind: edit.TargetFile, Path: source.Path(from)},
		edit.Args{edit.ArgDestination: to})
}

func TestMove(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the move and what the server said it implies", func(t *testing.T) {
			t.Parallel()
			// A server answers with the edits a move implies and never
			// with the move: it is being asked what would break, not
			// asked to do anything. So both are in the plan, and the
			// write path performs the relocation.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := moved(t, e, "a.fake", "b.fake")

			assert.NoError(t, err, "moving a file the server has an opinion about succeeds")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit, edit.ChangeMove},
				"the edits the server computed, then the move itself")
			assert.Equal(t, string(got.Items[1].To), "b.fake", "which names where the file goes")
		})

		t.Run("declines where the server does no file operations", func(t *testing.T) {
			t.Parallel()
			// Reported as no edits, a move would take the file and leave
			// every reference to it pointing at nothing. Most servers do
			// not answer this request.
			e := serving(t, modeMoveless, map[string]string{"a.fake": content})
			_, err := moved(t, e, "a.fake", "b.fake")

			assert.ErrorIs(t, err, engine.ErrDecline,
				"a request the server did not offer is passed on, not answered")
			assert.Contains(t, err.Error(), "willRenameFiles", "carrying what it would not do")
		})

		t.Run("says a move it was told nothing about covers less", func(t *testing.T) {
			t.Parallel()
			// A server that computed no edits has not shown it looked:
			// an empty answer is what both a file nothing refers to and
			// a server with no view of the workspace produce. metals
			// answers a move that way and publishes nothing to settle
			// it, and moving a file is refused rather than applied over
			// a claim that nothing referred to it.
			e := serving(t, modeSilentMove, map[string]string{"a.fake": content})
			got, err := moved(t, e, "a.fake", "b.fake")

			assert.NoError(t, err, "a move the server said nothing about is still a move")
			assert.Equal(t, got.Completeness, trust.ScopePartial,
				"and nothing may be read out of it having named no file")
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, got.Completeness),
				"so the policy will not admit it")
		})

		t.Run("refuses a move onto a file that is there", func(t *testing.T) {
			t.Parallel()
			// A move that overwrites is a file lost, reported as one path
			// changed.
			e := serving(t, modeDefault, map[string]string{
				"a.fake": content, "b.fake": content,
			})
			_, err := moved(t, e, "a.fake", "b.fake")

			assert.ErrorIs(t, err, engine.ErrRefuse, "the destination is taken")
			assert.Contains(t, err.Error(), "overwrite", "and the reason says so")
		})

		t.Run("refuses a file that is not there", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := moved(t, e, "gone.fake", "b.fake")

			assert.ErrorIs(t, err, engine.ErrRefuse, "there is nothing to move")
		})

		t.Run("refuses a destination outside the workspace", func(t *testing.T) {
			t.Parallel()
			// techne writes under its root and reports paths relative to
			// it, so a move reaching past it cannot be applied as
			// described.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := moved(t, e, "a.fake", "../b.fake")

			assert.ErrorIs(t, err, engine.ErrRefuse, "the destination is not in the workspace")
			assert.Contains(t, err.Error(), "outside", "and the reason says which end")
		})

		t.Run("moves a file whose name begins with two dots", func(t *testing.T) {
			t.Parallel()
			// Leaving the workspace is a path segment of two dots, not a
			// name that starts with them. A dotfile is inside.
			e := serving(t, modeDefault, map[string]string{
				"a.fake": content, "..config.fake": content,
			})
			got, err := moved(t, e, "..config.fake", "..settings.fake")

			assert.NoError(t, err, "a file called ..config is in the workspace")
			assert.Equal(t, string(got.Items[len(got.Items)-1].To), "..settings.fake",
				"and moves like any other")
		})

		t.Run("refuses a move to where the file already is", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := moved(t, e, "a.fake", "a.fake")

			assert.ErrorIs(t, err, engine.ErrRefuse, "there is nothing to do")
		})

		t.Run("refuses a move with nowhere to go", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := moved(t, e, "a.fake", "")

			assert.ErrorIs(t, err, engine.ErrRefuse, "a move needs a destination")
		})

		t.Run("refuses a target that is not a file", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := e.Plan(t.Context(), engine.Request{}, edit.MoveFile,
				edit.Target{Kind: edit.TargetSymbol, Symbol: "fake::Store"},
				edit.Args{edit.ArgDestination: "b.fake"})

			assert.ErrorIs(t, err, engine.ErrRefuse, "a file is what a move is pointed at")
		})
	})
}

// kinds is what each change in a plan does, in order.
func kinds(held []edit.Change) []edit.ChangeKind {
	out := make([]edit.ChangeKind, 0, len(held))
	for _, one := range held {
		out = append(out, one.Kind)
	}
	return out
}
