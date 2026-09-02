// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
)

// renaming asks for a declaration to be renamed, pointed at by identity.
func renaming(t *testing.T, e *lsp.Engine) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
		edit.RenameSymbol,
		edit.Target{Kind: edit.TargetSymbol, Symbol: subject(t, e, "Store")},
		edit.Args{edit.ArgNewName: "Vault"})
}

func TestRename(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("opens the files the uses are in before asking", func(t *testing.T) {
			t.Parallel()
			// A server computes a rename over the buffers the client is
			// holding. metals does exactly that: asked to rename a class
			// with only that class's file open, it rewrites that file
			// and leaves every other use of the class where it was,
			// while answering who uses the class perfectly well. So the
			// uses are asked for first and their files opened, which is
			// the state an editor would have been in.
			e := serving(t, modeOpened, map[string]string{
				"a.fake": content, "b.fake": content,
			})
			got, err := renaming(t, e)

			assert.NoError(t, err, "renaming succeeds")
			assert.Length(t, got.Items, 2,
				"the declaration's file and the file the server said uses it")
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"and the rename covers every use the server can name")
		})

		t.Run("reports a use it did not rewrite as coverage it does not have", func(t *testing.T) {
			t.Parallel()
			// The claim behind a rename is that every reference was
			// found. Measured against the server's own reference list, a
			// rename that misses one is short whatever it claims — and a
			// caller reading total coverage acts on it by deleting the
			// declaration.
			e := serving(t, modeShort, map[string]string{
				"a.fake": content, "b.fake": content,
			})
			got, err := renaming(t, e)

			assert.NoError(t, err, "a short rename is an answer, not a fault")
			assert.Equal(t, got.Completeness, trust.ScopePartial,
				"so nothing may be read out of it being every use")
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, got.Completeness),
				"and the policy will not admit an operation that rewrites references")
			assert.True(t, carries(got.Caveats, trust.CaveatIndexWarming),
				"the caveat names the use that was left")
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
