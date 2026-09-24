// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"math"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// roleWords pins the wire string of every role. A capability report renders
// these strings.
var roleWords = map[engine.Role]string{
	engine.RoleUnset:   "unset",
	engine.RoleOutline: "outline",
	engine.RoleSearch:  "search",
	engine.RoleResolve: "resolve",
	engine.RoleRelate:  "relate",
	engine.RolePlan:    "plan",
	engine.RoleFormat:  "format",
	engine.RoleCheck:   "check",
	engine.RoleVerify:  "verify",
	engine.RoleIndex:   "index",
}

// costWords pins the wire string of every cost.
var costWords = map[engine.Cost]string{
	engine.CostMemory:  "memory",
	engine.CostParse:   "parse",
	engine.CostAnalyze: "analyze",
	engine.CostSession: "session",
	engine.CostProcess: "process",
}

func TestRole(t *testing.T) {
	t.Parallel()

	t.Run("Role.String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the wire string of every role", func(t *testing.T) {
			t.Parallel()
			for role, want := range roleWords {
				assert.Equal(t, role.String(), want, want)
			}
		})

		t.Run("returns unset for an undeclared role", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, engine.Role(200).String(), "unset", "Role(200)")
		})
	})

	t.Run("Roles", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every declared role except RoleUnset", func(t *testing.T) {
			t.Parallel()
			var declared []engine.Role
			for i := range math.MaxUint8 + 1 {
				if role := engine.Role(i); role.String() != engine.RoleUnset.String() {
					declared = append(declared, role)
				}
			}
			assert.Equal(t, engine.Roles(), declared, "roles")
		})

		t.Run("lists only roles the catalogue can select", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, complete{name: "complete", fidelity: trust.Resolved})
			for _, role := range engine.Roles() {
				assert.Length(t, c.For(t.Context(), fixture, role), 1, role.String())
			}
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("orders costs from cheapest to most expensive", func(t *testing.T) {
			t.Parallel()
			rising := []engine.Cost{
				engine.CostMemory, engine.CostParse, engine.CostAnalyze,
				engine.CostSession, engine.CostProcess,
			}
			assert.Pairwise(t, rising, func(cheaper, dearer engine.Cost) bool {
				return cheaper < dearer
			}, "costs")
		})
	})

	t.Run("Cost.String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the wire string of every cost", func(t *testing.T) {
			t.Parallel()
			for cost, want := range costWords {
				assert.Equal(t, cost.String(), want, want)
			}
		})

		t.Run("returns process for an undeclared cost", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, engine.Cost(200).String(), "process", "Cost(200)")
		})
	})
}
