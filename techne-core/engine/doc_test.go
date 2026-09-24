// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// server is a complete engine that declares Resolved for RoleRelate and
// Syntactic for every other role, at CostSession.
type server struct{ complete }

func (server) Fidelity(r engine.Role) trust.Fidelity {
	if r == engine.RoleRelate {
		return trust.Resolved
	}
	return trust.Syntactic
}

func (server) Cost(engine.Role) engine.Cost { return engine.CostSession }

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Catalog.For", func(t *testing.T) {
		t.Parallel()

		lsp := server{complete{name: "server"}}
		parser := complete{name: "parser", fidelity: trust.Syntactic, cost: engine.CostParse}

		t.Run("selects the parser first for RoleOutline", func(t *testing.T) {
			t.Parallel()
			got := catalog(t, lsp, parser).For(t.Context(), fixture, engine.RoleOutline)
			assert.Equal(t, names(got), []string{"parser", "server"}, "engines")
		})

		t.Run("selects the server first for RoleRelate", func(t *testing.T) {
			t.Parallel()
			got := catalog(t, lsp, parser).For(t.Context(), fixture, engine.RoleRelate)
			assert.Equal(t, names(got), []string{"server", "parser"}, "engines")
		})
	})
}
