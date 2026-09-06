// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/go/checker"
)

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration that names no language", func(t *testing.T) {
			t.Parallel()
			_, err := checker.New(t.TempDir(), lang.Declaration{})
			assert.HasError(t, err, "an engine answers about one language and must know which")
		})

		t.Run("refuses a root that is not a directory", func(t *testing.T) {
			t.Parallel()
			// The loader runs the go command against a directory, so a
			// tree that is nowhere has no packages to load. Refused here
			// rather than on the first call.
			held := filepath.Join(workspace(t, whole()), "store.go")

			_, err := checker.New(held, golang.Declaration())
			assert.HasError(t, err, "a file is not a workspace")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("is the toolchain rather than the package", func(t *testing.T) {
			t.Parallel()
			// Told the Go type checker answered, a caller knows the
			// answer is as good as the build is and that no server was
			// involved.
			assert.Equal(t, serving(t, whole()).Name(), "go/types", "the mechanism, named")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is resolved for what it binds", func(t *testing.T) {
			t.Parallel()
			e := serving(t, whole())
			assert.Equal(t, e.Fidelity(engine.RoleRelate), trust.Resolved,
				"it binds names through the same checker the compiler runs")
		})

		t.Run("is nothing for a role a parser owns", func(t *testing.T) {
			t.Parallel()
			// Claiming a tier for outlining would win the catalogue's
			// sort and type-check a module to do what a parser does in a
			// millisecond.
			e := serving(t, whole())
			assert.Equal(t, e.Fidelity(engine.RoleOutline), trust.None,
				"an undeclared role reaches nothing")
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("is a session rather than what the first call costs", func(t *testing.T) {
			t.Parallel()
			// Type-checking a module is seconds and the result is kept.
			// Priced at the first call, a catalogue would route around
			// the only engine that can answer at all where no server is
			// installed.
			assert.Equal(t, serving(t, whole()).Cost(engine.RoleRelate), engine.CostSession,
				"dear once and cheap after")
		})
	})

	t.Run("the roles it serves", func(t *testing.T) {
		t.Parallel()

		t.Run("are the ones a type checker answers better", func(t *testing.T) {
			t.Parallel()
			var held any = serving(t, whole())

			for _, one := range []struct {
				role   engine.Role
				serves bool
			}{
				{engine.RoleResolve, true},
				{engine.RoleRelate, true},
				{engine.RoleVerify, true},
				{engine.RoleCheck, true},
				{engine.RoleOutline, false},
				{engine.RoleSearch, false},
				{engine.RolePlan, false},
				{engine.RoleFormat, false},
				{engine.RoleIndex, false},
			} {
				_, serves := satisfies(held, one.role)
				assert.Equal(t, serves, one.serves,
					"the port is present exactly where a type checker beats a parser: "+
						one.role.String())
			}
		})
	})
}

// satisfies reports whether an engine implements the port for a role.
func satisfies(held any, role engine.Role) (any, bool) {
	switch role {
	case engine.RoleOutline:
		one, ok := held.(engine.Outliner)
		return one, ok
	case engine.RoleSearch:
		one, ok := held.(engine.Searcher)
		return one, ok
	case engine.RoleResolve:
		one, ok := held.(engine.Resolver)
		return one, ok
	case engine.RoleRelate:
		one, ok := held.(engine.Relator)
		return one, ok
	case engine.RolePlan:
		one, ok := held.(engine.Planner)
		return one, ok
	case engine.RoleFormat:
		one, ok := held.(engine.Formatter)
		return one, ok
	case engine.RoleCheck:
		one, ok := held.(engine.Checker)
		return one, ok
	case engine.RoleVerify:
		one, ok := held.(engine.Verifier)
		return one, ok
	case engine.RoleIndex:
		one, ok := held.(engine.Indexer)
		return one, ok
	}
	return nil, false
}
