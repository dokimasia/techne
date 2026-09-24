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

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("returns resolved for the four roles of the package comment", func(t *testing.T) {
			t.Parallel()
			held := checker.Binding()
			assert.Length(t, held, 4, "the roles of Binding")
			for _, role := range []engine.Role{
				engine.RoleResolve, engine.RoleRelate, engine.RoleVerify, engine.RoleCheck,
			} {
				assert.Equal(t, held[role], trust.Resolved, "the tier of "+role.String())
			}
		})

		t.Run("returns no tier for a role that the parser serves", func(t *testing.T) {
			t.Parallel()
			held := checker.Binding()
			for _, role := range []engine.Role{
				engine.RoleOutline, engine.RoleSearch, engine.RolePlan, engine.RoleFormat, engine.RoleIndex,
			} {
				assert.Equal(t, held[role], trust.None, "the tier of "+role.String())
			}
		})
	})
}
