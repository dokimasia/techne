// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
)

func TestRole(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of the role", func(t *testing.T) {
			t.Parallel()
			// A capability report names roles, so these strings reach a
			// caller and are pinned rather than derived.
			for role, want := range map[engine.Role]string{
				engine.RoleUnset:   "unset",
				engine.RoleOutline: "outline",
				engine.RoleSearch:  "search",
				engine.RoleResolve: "resolve",
				engine.RoleRelate:  "relate",
				engine.RolePlan:    "plan",
				engine.RoleFormat:  "format",
				engine.RoleVerify:  "verify",
				engine.RoleIndex:   "index",
			} {
				assert.Equal(t, role.String(), want,
					"a capability report names roles, so these strings reach a caller")
			}
		})

		t.Run("falls back to unset outside the set", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, engine.Role(200).String(), "unset",
				"a role outside the set reads as unset rather than as an empty name")
		})
	})

	t.Run("Roles", func(t *testing.T) {
		t.Parallel()

		t.Run("names a role for every port", func(t *testing.T) {
			t.Parallel()
			// Eight ports, eight roles. A port added without a role
			// cannot be selected for, and a role without a port can be
			// asked for and never served.
			assert.Length(t, engine.Roles(), 8,
				"a port with no role cannot be selected for, and a role with no port can be asked for and never served")
		})

		t.Run("excludes the unset role", func(t *testing.T) {
			t.Parallel()
			for _, r := range engine.Roles() {
				assert.NotEqual(t, r, engine.RoleUnset, "no port serves the unset role")
			}
		})

		t.Run("names each role once", func(t *testing.T) {
			t.Parallel()
			seen := map[engine.Role]bool{}
			for _, r := range engine.Roles() {
				assert.False(t, seen[r], "a role listed twice would be reported twice")
				seen[r] = true
			}
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("rises so a catalogue sorts cheapest first", func(t *testing.T) {
			t.Parallel()
			ordered := []engine.Cost{
				engine.CostMemory, engine.CostParse, engine.CostAnalyze,
				engine.CostSession, engine.CostProcess,
			}
			assert.Pairwise(t, ordered, func(earlier, later engine.Cost) bool {
				return earlier < later
			}, "the order rises, so a catalogue takes the cheapest of two equals first")
		})

		t.Run("prices a session below a subprocess", func(t *testing.T) {
			t.Parallel()
			// A language server is expensive once and cheap afterwards.
			// Sorted above a per-call subprocess, a caller would avoid
			// the fastest engine it has.
			assert.True(t, engine.CostSession < engine.CostProcess,
				"a server is expensive once and cheap after, so pricing it per call would hide the fastest engine")
		})
	})
}
