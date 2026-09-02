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
)

// A server states at initialise what it answers, and refuses everything
// else with an error. An error stops the whole call, so a question a
// server simply does not answer would come back as a broken read rather
// than as a question to put to something else.
//
// pyright is the case that found this: it does not answer
// textDocument/implementation, and asking anyway turned "who implements
// this" into a failure.
func TestCapable(t *testing.T) {
	t.Parallel()

	t.Run("a request the server did not offer", func(t *testing.T) {
		t.Parallel()

		t.Run("is declined rather than asked and failed", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeThin, map[string]string{"a.fake": content})
			_, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.ErrorIs(t, err, engine.ErrDecline,
				"another engine gets a turn, rather than the read breaking")
			assert.Contains(t, err.Error(), "textDocument/references",
				"and the reason names what this server does not answer")
		})

		t.Run("is declined for every role that has one", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeThin, map[string]string{"a.fake": content})

			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"},
				source.Position{Line: 2, Column: 5})
			assert.ErrorIs(t, err, engine.ErrDecline, "definition is not offered")

			_, err = e.Search(t.Context(), engine.Request{Scope: "."},
				engine.Query{Text: "St"})
			assert.ErrorIs(t, err, engine.ErrDecline, "nor is the workspace query")

			_, err = e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.CalledBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "nor the call hierarchy")
		})
	})

	t.Run("a rename on a server that does not prepare", func(t *testing.T) {
		t.Parallel()

		t.Run("goes ahead without asking", func(t *testing.T) {
			t.Parallel()
			// Asking a server that does not answer it refuses every
			// rename it would have done, because the error is
			// indistinguishable from the position being unrenameable.
			e := serving(t, modeThin, map[string]string{"a.fake": content})
			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: subject(t, e, "Store")},
				edit.Args{edit.ArgNewName: "Vault"})

			assert.NoError(t, err, "a server that renames is asked to rename")
			assert.Length(t, got.Items, 1, "and the file it named comes back")
		})
	})
}
