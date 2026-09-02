// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// TestDoc covers the contracts the package comment states, which no one
// file owns.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("one server, kept warm", func(t *testing.T) {
		t.Parallel()

		t.Run("so a later question costs no start", func(t *testing.T) {
			t.Parallel()
			// The measurement behind the claim: initialise answers in
			// 26ms and a first document symbol in 90ms, against one or
			// two for every one after. Started per call, every question
			// pays the first one's price.
			e, started := counting(t)
			for range 3 {
				_, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
				assert.NoError(t, err, "each question is answered")
			}
			_, err := e.Search(t.Context(), engine.Request{Scope: "."},
				engine.Query{Text: "St"})
			assert.NoError(t, err, "and so is a question of another kind")

			assert.Equal(t, started(), 1, "by the one server all four went to")
		})
	})

	t.Run("a tier per role, declared by the language", func(t *testing.T) {
		t.Parallel()

		t.Run("so a server can bind strongly and outline weakly", func(t *testing.T) {
			t.Parallel()
			// A server outlines one file no better than a parser does, at
			// a thousandth of the speed. One tier for the engine would
			// make an outline start a process to do worse.
			held := pretending(modeDefault)
			held.Serves[engine.RoleOutline] = trust.Syntactic

			e := servingAs(t, held, map[string]string{"a.fake": content})
			assert.True(t, e.Fidelity(engine.RoleResolve) > e.Fidelity(engine.RoleOutline),
				"one tier per engine would force a server to claim the weaker of the two for both")
		})
	})

	t.Run("declared whether or not it is installed", func(t *testing.T) {
		t.Parallel()

		t.Run("so a language does not vanish with its server", func(t *testing.T) {
			t.Parallel()
			// Told nothing, a caller concludes the language cannot be
			// served at all. Told the server is missing, it knows what to
			// install.
			e := buildingOn(t, missing())

			assert.Equal(t, string(e.Language()), "fake", "the language is still declared")
			assert.HasError(t, e.Available(t.Context()), "and reports why it cannot answer")
		})
	})

	t.Run("what it counts in", func(t *testing.T) {
		t.Parallel()

		t.Run("is bytes, whichever way a position crossed the boundary", func(t *testing.T) {
			t.Parallel()
			// The protocol counts UTF-16 code units. A span taken as
			// bytes lands in the middle of a rune, and the edit computed
			// from it writes over half a character.
			got, err := serving(t, modeUnicode, map[string]string{"a.fake": unicode}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			assert.Equal(t, got.Items[0].Snippet, "Störe",
				"a whole name rather than the bytes either side of one")
		})
	})

	t.Run("an answer states what it is not", func(t *testing.T) {
		t.Parallel()

		t.Run("so nothing found is never read as nothing there", func(t *testing.T) {
			t.Parallel()
			// The distinction the whole package turns on. A scope holding
			// none of this language's files says nothing about the
			// language, and must not lower what an engine beside it is
			// worth.
			e := serving(t, modeDefault, map[string]string{"notes.md": "# notes\n"})

			outlined, err := e.Outline(t.Context(), engine.Request{Scope: "."})
			assert.NoError(t, err, "a scope with nothing to read is not a fault")
			assert.True(t, outlined.Skipped, "the outline says it read nothing")

			verified, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "nor for a gate")
			assert.True(t, verified.Skipped,
				"and so does the gate, rather than reporting the scope clean")
		})
	})
}
