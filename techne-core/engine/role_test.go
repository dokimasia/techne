// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

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
				if got := role.String(); got != want {
					t.Errorf("Role(%d).String() = %q, want %q", role, got, want)
				}
			}
		})

		t.Run("falls back to unset outside the set", func(t *testing.T) {
			t.Parallel()
			if got := engine.Role(200).String(); got != "unset" {
				t.Errorf("Role(200).String() = %q, want %q", got, "unset")
			}
		})
	})

	t.Run("Roles", func(t *testing.T) {
		t.Parallel()

		t.Run("names a role for every port", func(t *testing.T) {
			t.Parallel()
			// Eight ports, eight roles. A port added without a role
			// cannot be selected for, and a role without a port can be
			// asked for and never served.
			if got := len(engine.Roles()); got != 8 {
				t.Errorf("Roles() has %d entries, want one per port", got)
			}
		})

		t.Run("excludes the unset role", func(t *testing.T) {
			t.Parallel()
			for _, r := range engine.Roles() {
				if r == engine.RoleUnset {
					t.Error("Roles() includes RoleUnset, which no port serves")
				}
			}
		})

		t.Run("names each role once", func(t *testing.T) {
			t.Parallel()
			seen := map[engine.Role]bool{}
			for _, r := range engine.Roles() {
				if seen[r] {
					t.Errorf("role %q is listed twice", r)
				}
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
			for i := 1; i < len(ordered); i++ {
				if ordered[i-1] >= ordered[i] {
					t.Errorf("cost %d is not below %d", ordered[i-1], ordered[i])
				}
			}
		})

		t.Run("prices a session below a subprocess", func(t *testing.T) {
			t.Parallel()
			// A language server is expensive once and cheap afterwards.
			// Sorted above a per-call subprocess, a caller would avoid
			// the fastest engine it has.
			if engine.CostSession >= engine.CostProcess {
				t.Error("CostSession must sort below CostProcess")
			}
		})
	})
}
