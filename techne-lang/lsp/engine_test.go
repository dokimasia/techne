// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
)

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration that names no language", func(t *testing.T) {
			t.Parallel()
			_, err := lsp.New(t.TempDir(), lang.Declaration{}, pretending(""))
			assert.HasError(t, err, "an engine answers about one language and must know which")
		})

		t.Run("refuses a root that is not a directory", func(t *testing.T) {
			t.Parallel()
			// A language server is a process that opens files itself and
			// cannot be handed a tree that is not on disk. Refused here
			// rather than on the first call.
			held := filepath.Join(workspace(t, map[string]string{"a.fake": content}), "a.fake")

			_, err := lsp.New(held, declared(), pretending(""))
			assert.HasError(t, err, "a file is not a workspace")
		})

		t.Run("refuses a server declaration that cannot be run", func(t *testing.T) {
			t.Parallel()
			// Checked when a module registers rather than when a call
			// arrives: a server with no language id opens every file
			// under an empty name and is answered about nothing.
			held := pretending("")
			held.LanguageID = ""

			_, err := lsp.New(t.TempDir(), declared(), held)
			assert.HasError(t, err, "a declaration missing what a server needs is refused")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("is the server rather than the language", func(t *testing.T) {
			t.Parallel()
			// Told gopls answered, a caller knows what to install, what
			// to upgrade and whose release notes to read. Told "go", it
			// knows none of that.
			e := serving(t, "", map[string]string{"a.fake": content})
			assert.Equal(t, e.Name(), "fake", "the server's own name")
			assert.Equal(t, string(e.Language()), "fake", "beside the language it answers about")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is what the language module declared, per role", func(t *testing.T) {
			t.Parallel()
			e := serving(t, "", map[string]string{"a.fake": content})
			assert.Equal(t, e.Fidelity(engine.RoleResolve), trust.Resolved,
				"the tier the declaration claimed")
		})

		t.Run("is nothing for a role the declaration left out", func(t *testing.T) {
			t.Parallel()
			// A server claiming a tier for a role nobody declared would
			// win a catalogue's sort for work it cannot do.
			e := serving(t, "", map[string]string{"a.fake": content})
			assert.Equal(t, e.Fidelity(engine.RoleFormat), trust.None,
				"an undeclared role reaches nothing")
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("is a session rather than what the first call costs", func(t *testing.T) {
			t.Parallel()
			// Priced at the first call, a catalogue would prefer a parser
			// for every question, including the ones only a server can
			// answer.
			e := serving(t, "", map[string]string{"a.fake": content})
			assert.Equal(t, e.Cost(engine.RoleResolve), engine.CostSession,
				"dear once and cheap after")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("says what is missing rather than only that something is", func(t *testing.T) {
			t.Parallel()
			// A missing tool is a problem a caller can act on and a
			// missing capability is one to route around. Told only "no",
			// the two look the same.
			e, err := lsp.New(t.TempDir(), declared(), missing())
			assert.NoError(t, err, "a declaration for a server that is not here is still valid")

			err = e.Available(t.Context())
			assert.HasError(t, err, "the server is not on this machine")
			assert.Contains(t, err.Error(), "PATH", "and the reason names what to fix")
		})

		t.Run("passes for a server that is here", func(t *testing.T) {
			t.Parallel()
			e := serving(t, "", map[string]string{"a.fake": content})
			assert.NoError(t, e.Available(t.Context()), "the fake is this test binary")
		})
	})

	t.Run("the roles it serves", func(t *testing.T) {
		t.Parallel()

		t.Run("are the ones with a request behind them", func(t *testing.T) {
			t.Parallel()
			// A role is declined by lacking a method rather than by
			// returning an error, so what this engine satisfies is the
			// whole of what a catalogue can select it for.
			var held any = serving(t, "", map[string]string{"a.fake": content})

			for _, one := range []struct {
				role   engine.Role
				serves bool
			}{
				{engine.RoleOutline, true},
				{engine.RoleSearch, true},
				{engine.RoleResolve, true},
				{engine.RoleRelate, true},
				{engine.RolePlan, true},
				{engine.RoleVerify, true},
				{engine.RoleFormat, false},
				{engine.RoleCheck, false},
				{engine.RoleIndex, false},
			} {
				assert.Equal(t, satisfies(held, one.role), one.serves,
					"the port is present exactly where the protocol has a request: "+
						one.role.String())
			}
		})
	})
}

// satisfies reports whether an engine implements the port for a role.
func satisfies(held any, role engine.Role) bool {
	switch role {
	case engine.RoleOutline:
		_, ok := held.(engine.Outliner)
		return ok
	case engine.RoleSearch:
		_, ok := held.(engine.Searcher)
		return ok
	case engine.RoleResolve:
		_, ok := held.(engine.Resolver)
		return ok
	case engine.RoleRelate:
		_, ok := held.(engine.Relator)
		return ok
	case engine.RolePlan:
		_, ok := held.(engine.Planner)
		return ok
	case engine.RoleVerify:
		_, ok := held.(engine.Verifier)
		return ok
	case engine.RoleFormat:
		_, ok := held.(engine.Formatter)
		return ok
	case engine.RoleCheck:
		_, ok := held.(engine.Checker)
		return ok
	case engine.RoleIndex:
		_, ok := held.(engine.Indexer)
		return ok
	}
	return false
}
