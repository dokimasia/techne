// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/go/checker"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("what this engine claims", func(t *testing.T) {
		t.Parallel()

		t.Run("is four roles and no more", func(t *testing.T) {
			t.Parallel()
			// The package comment names them. A role added to the binding
			// without a port to serve it would be advertised and never
			// selectable, and one added without the comment moving would
			// leave the two disagreeing.
			held := checker.Binding()
			assert.Length(t, held, 4, "resolve, relate, verify and check")
			for _, role := range []engine.Role{
				engine.RoleResolve, engine.RoleRelate, engine.RoleVerify, engine.RoleCheck,
			} {
				assert.Equal(t, held[role], trust.Resolved,
					"it binds names through the compiler's own checker: "+role.String())
			}
		})

		t.Run("leaves a parser the roles a parser does better", func(t *testing.T) {
			t.Parallel()
			// Claiming a tier for outlining would win the catalogue's
			// sort and type-check a module to do what a parser does in a
			// millisecond.
			held := checker.Binding()
			for _, role := range []engine.Role{
				engine.RoleOutline, engine.RoleSearch, engine.RolePlan,
				engine.RoleFormat, engine.RoleIndex,
			} {
				assert.Equal(t, held[role], trust.None,
					"nothing is claimed for a role a parser owns: "+role.String())
			}
		})

		t.Run("is the same tier a language server claims", func(t *testing.T) {
			t.Parallel()
			// Which is what makes the order between them the order they
			// were registered, and the language module registers the
			// server first.
			assert.Equal(t, checker.Binding()[engine.RoleRelate], trust.Resolved,
				"two engines that both bind names claim the same evidence")
		})
	})
}
