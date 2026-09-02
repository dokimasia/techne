// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/lsp"
)

// renaming asks for the one operation this engine plans, pointed at a
// declaration by identity.
func renaming(t *testing.T, e *lsp.Engine) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
		edit.RenameSymbol,
		edit.Target{Kind: edit.TargetSymbol, Symbol: subject(t, e, "Store")},
		edit.Args{edit.ArgNewName: "Vault"})
}

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a workspace edit sent as a map of files", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := renaming(t, e)

			assert.NoError(t, err, "planning a rename succeeds")
			assert.Length(t, got.Items, 1, "one file to change")
			assert.Equal(t, got.Items[0].Kind, edit.ChangeEdit, "by rewriting ranges in it")
			assert.Length(t, got.Items[0].Edits, 2, "one per site the name reaches")
		})

		t.Run("reads a workspace edit sent as an ordered list", func(t *testing.T) {
			t.Parallel()
			// The newer shape can carry file operations the map cannot
			// express, and a rename that renames the file is exactly
			// when a server sends it. Reading only the map silently
			// drops the move.
			e := serving(t, modeOrdered, map[string]string{"a.fake": content})
			got, err := renaming(t, e)

			assert.NoError(t, err, "planning a rename succeeds")
			assert.Length(t, got.Items, 2, "the edits and the move the server asked for")

			var moved bool
			for _, one := range got.Items {
				moved = moved || one.Kind == edit.ChangeMove
			}
			assert.True(t, moved, "the file operation is carried, not dropped")
		})

		t.Run("orders the edits the write path applies in one pass", func(t *testing.T) {
			t.Parallel()
			// A server sends them in whatever order it found them. The
			// write path walks them once and relies on the order.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := renaming(t, e)

			assert.NoError(t, err, "planning succeeds")
			assert.Pairwise(t, got.Items[0].Edits, func(earlier, later edit.TextEdit) bool {
				return earlier.Span.Start.Offset < later.Span.Start.Offset
			}, "sorted by where each starts, whatever order they arrived in")
		})

		t.Run("refuses a workspace edit whose ranges overlap", func(t *testing.T) {
			t.Parallel()
			// Applied in one pass they would write over each other. A
			// change that cannot be applied as described must not be
			// handed on as though it could.
			e := serving(t, modeOverlapping, map[string]string{"a.fake": content})
			_, err := renaming(t, e)

			assert.HasError(t, err, "overlapping edits are refused rather than applied wrongly")
			assert.Contains(t, err.Error(), "overlapping", "and the reason says so")
		})

		t.Run("refuses a position the server says cannot be renamed", func(t *testing.T) {
			t.Parallel()
			// Skipping the question turns a keyword or a literal into a
			// rename that reports no edits, which reads as a rename that
			// had nothing to do.
			e := serving(t, modeUnnameable, map[string]string{"a.fake": content})
			_, err := renaming(t, e)

			assert.ErrorIs(t, err, engine.ErrRefuse,
				"a refusal a caller can act on, not an empty plan")
		})

		t.Run("refuses a rename with no new name", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: subject(t, e, "Store")},
				edit.Args{})

			assert.ErrorIs(t, err, engine.ErrRefuse, "the argument is required")
		})

		t.Run("declines an operation no request answers", func(t *testing.T) {
			t.Parallel()
			// Declining lets a parser that can do it have a turn.
			// Approximating it would put a plan behind the write gate
			// that no type checker computed.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.ExtractFunction,
				edit.Target{Kind: edit.TargetSymbol, Symbol: subject(t, e, "Store")},
				edit.Args{})

			assert.ErrorIs(t, err, engine.ErrDecline,
				"an operation with nothing behind it declines, so another engine gets a turn")
		})

		t.Run("refuses a rename that reaches outside the workspace", func(t *testing.T) {
			t.Parallel()
			// techne applies changes under its root and reports them
			// relative to it, so a plan reaching past it cannot be
			// applied as described. Half a rename is worse than none.
			e := reaching(t, map[string]string{"a.fake": content})
			_, err := renaming(t, e)

			assert.ErrorIs(t, err, engine.ErrRefuse,
				"a change techne cannot apply is refused rather than half-applied")
			assert.Contains(t, err.Error(), "outside the workspace", "and the reason says why")
		})

		t.Run("plans from a span as readily as from an identity", func(t *testing.T) {
			t.Parallel()
			// A caller that has already resolved which declaration it
			// meant should not have to name it a second way.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			outlined, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the case can find what it points at")
			one, _ := named(outlined.Items, "Store")

			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: one.Span},
				edit.Args{edit.ArgNewName: "Vault"})

			assert.NoError(t, err, "planning from a span succeeds")
			assert.Length(t, got.Items, 1, "and reaches the same file")
		})
	})
}
