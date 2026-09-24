// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
	"go.lsp.dev/protocol"
)

func TestIdentity(t *testing.T) {
	t.Parallel()

	identities := map[string]protocol.LanguageKind{
		lsp.IdentityC:          protocol.LanguageKindC,
		lsp.IdentityCSharp:     protocol.LanguageKindCSharp,
		lsp.IdentityGo:         protocol.LanguageKindGo,
		lsp.IdentityJava:       protocol.LanguageKindJava,
		lsp.IdentityJavaScript: protocol.LanguageKindJavaScript,
		lsp.IdentityPython:     protocol.LanguageKindPython,
		lsp.IdentityRuby:       protocol.LanguageKindRuby,
		lsp.IdentityRust:       protocol.LanguageKindRust,
		lsp.IdentityScala:      protocol.LanguageKindScala,
		lsp.IdentityTypeScript: protocol.LanguageKindTypeScript,
	}

	t.Run("Identity", func(t *testing.T) {
		t.Parallel()

		t.Run("equals the language identifier of the protocol", func(t *testing.T) {
			t.Parallel()
			for identity, kind := range identities {
				assert.Equal(t, identity, string(kind), "the identifier of "+identity)
			}
		})

		t.Run("differs for each of the ten languages", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, identities, 10, "the distinct identifiers of the ten languages")
		})
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("claims Resolved for the six roles an Engine serves", func(t *testing.T) {
			t.Parallel()
			for _, role := range []engine.Role{
				engine.RoleResolve, engine.RoleRelate, engine.RolePlan,
				engine.RoleFormat, engine.RoleCheck, engine.RoleVerify,
			} {
				assert.Equal(t, lsp.Binding()[role], trust.Resolved, "the tier of "+role.String())
			}
		})

		t.Run("claims no tier for outline or search", func(t *testing.T) {
			t.Parallel()
			for _, role := range []engine.Role{engine.RoleOutline, engine.RoleSearch} {
				_, claimed := lsp.Binding()[role]
				assert.False(t, claimed, "Binding claims "+role.String())
			}
		})

		t.Run("returns a new map for each call", func(t *testing.T) {
			t.Parallel()
			first := lsp.Binding()
			delete(first, engine.RolePlan)
			_, kept := lsp.Binding()[engine.RolePlan]
			assert.True(t, kept, "the second Binding contains RolePlan")
		})

		t.Run("returns a map that Server.Valid accepts", func(t *testing.T) {
			t.Parallel()
			server := lsp.Server{
				Name: "any", Command: []string{"any"}, LanguageID: lsp.IdentityGo, Serves: lsp.Binding(),
			}
			assert.NoError(t, server.Valid(), "Valid of a declaration with Binding")
		})
	})
}
