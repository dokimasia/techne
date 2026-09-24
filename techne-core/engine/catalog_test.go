// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// catalog returns a catalogue of engines, registered in order.
func catalog(t *testing.T, engines ...engine.Engine) *engine.Catalog {
	t.Helper()
	c := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, c.Add(e), "Add "+e.Name())
	}
	return c
}

// names returns the names of engines, in order.
func names(engines []engine.Engine) []string {
	out := make([]string, 0, len(engines))
	for _, e := range engines {
		out = append(out, e.Name())
	}
	return out
}

func TestCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a second engine with one name and language", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			assert.NoError(t, c.Add(fake{name: "parser"}), "first Add")
			assert.HasError(t, c.Add(fake{name: "parser"}), "second Add")
		})

		t.Run("accepts one name for two languages", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			assert.NoError(t, c.Add(fake{name: "shared"}), "fixture")
			assert.NoError(t, c.Add(fake{name: "shared", language: other}), "other")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []engine.Engine
			want []string
		}{
			{
				name: "orders engines by fidelity",
				give: []engine.Engine{
					fake{name: "parser", fidelity: trust.Syntactic, cost: engine.CostParse},
					fake{name: "checker", fidelity: trust.Resolved, cost: engine.CostAnalyze},
				},
				want: []string{"checker", "parser"},
			},
			{
				name: "orders engines of equal fidelity by cost",
				give: []engine.Engine{
					fake{name: "parser", fidelity: trust.Syntactic, cost: engine.CostParse},
					fake{name: "index", fidelity: trust.Syntactic, cost: engine.CostMemory},
				},
				want: []string{"index", "parser"},
			},
			{
				name: "keeps the registration order of equal engines",
				give: []engine.Engine{
					fake{name: "first", fidelity: trust.Syntactic, cost: engine.CostParse},
					fake{name: "second", fidelity: trust.Syntactic, cost: engine.CostParse},
				},
				want: []string{"first", "second"},
			},
			{
				name: "skips an engine that declares None for the role",
				give: []engine.Engine{fake{name: "silent", fidelity: trust.None}},
				want: []string{},
			},
			{
				name: "skips an engine whose Available returns an error",
				give: []engine.Engine{
					complete{name: "server", fidelity: trust.Resolved, unusable: errors.New("server: not on PATH")},
					fake{name: "parser", fidelity: trust.Syntactic},
				},
				want: []string{"parser"},
			},
			{
				name: "selects an engine that does not implement Available",
				give: []engine.Engine{fake{name: "parser", fidelity: trust.Syntactic}},
				want: []string{"parser"},
			},
			{
				name: "skips an engine of another language",
				give: []engine.Engine{fake{name: "parser", language: other, fidelity: trust.Syntactic}},
				want: []string{},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := catalog(t, tt.give...).For(t.Context(), fixture, engine.RoleOutline)
				assert.Equal(t, names(got), tt.want, "engines")
			})
		}

		t.Run("skips an engine that does not implement the port", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, fake{name: "parser", fidelity: trust.Syntactic})
			assert.Empty(t, c.For(t.Context(), fixture, engine.RoleVerify), "engines")
		})
	})

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every role an engine offers", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, complete{name: "complete", fidelity: trust.Resolved})
			var roles []engine.Role
			for _, capability := range c.Capabilities(t.Context()) {
				roles = append(roles, capability.Role)
			}
			assert.Equal(t, roles, engine.Roles(), "roles")
		})

		t.Run("omits a role the engine declares None for", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, fake{name: "silent", fidelity: trust.None})
			assert.Empty(t, c.Capabilities(t.Context()), "capabilities")
		})

		t.Run("reports the Available error of an engine", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, complete{
				name: "server", fidelity: trust.Resolved, unusable: errors.New("server: not on PATH"),
			})
			for _, capability := range c.Capabilities(t.Context()) {
				assert.False(t, capability.Available, capability.Role.String())
				assert.Equal(t, capability.Unavailable, "server: not on PATH", capability.Role.String())
			}
		})

		t.Run("calls Available once per engine", func(t *testing.T) {
			t.Parallel()
			checks := 0
			c := catalog(t, complete{name: "server", fidelity: trust.Resolved, checks: &checks})
			c.Capabilities(t.Context())
			assert.Equal(t, checks, 1, "Available calls")
		})
	})
}
