// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/mock"
)

// TestDoc covers the claim the package comment makes: every port core
// declares is answered here, which is what nothing techne ships can do.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the roles", func(t *testing.T) {
		t.Parallel()

		t.Run("are every one a read or a write goes through", func(t *testing.T) {
			t.Parallel()
			// The tools for resolve, relations, verify and any operation
			// that rewrites references had nothing but a refusal to be
			// tested against before this. A role dropped from here is a
			// tool that quietly stops being exercised.
			catalogue := engine.NewCatalog()
			assert.NoError(t, catalogue.Add(built(t)), "the engine registers")

			for _, role := range []engine.Role{
				engine.RoleOutline, engine.RoleSearch, engine.RoleResolve,
				engine.RoleRelate, engine.RolePlan, engine.RoleCheck, engine.RoleVerify,
			} {
				assert.Length(t, catalogue.For(t.Context(), mock.Language, role), 1,
					"a role this language stopped serving is a tool nothing drives")
			}
		})
	})

	t.Run("what it claims", func(t *testing.T) {
		t.Parallel()

		t.Run("is what it does", func(t *testing.T) {
			t.Parallel()
			// Resolved and total, and both are true within this
			// language: a use names a declaration, and the workspace is
			// read whole to find it. An engine claiming more than it
			// does would make every refusal it drives a lie.
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src/store.mock"},
				storeID(t), 0)
			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the whole workspace was read")

			resolved, err := built(t).Resolve(t.Context(),
				engine.Request{Scope: "src/client.mock"}, source.Position{Line: 2, Column: 6})
			assert.NoError(t, err, "resolving succeeds")
			assert.NotEmpty(t, resolved.Items,
				"a scope of one file still resolves a name declared in another")
		})
	})
}
