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

	t.Run("the identity of a language", func(t *testing.T) {
		t.Parallel()

		t.Run("is the one the protocol names it by", func(t *testing.T) {
			t.Parallel()
			// A module declaring "golang" opens every file under a name
			// no server recognises and is answered about nothing, which
			// in a tool whose job includes reporting that it found
			// nothing is the hardest failure to notice. Taken from the
			// protocol's own constants rather than written out again.
			for held, want := range map[string]protocol.LanguageKind{
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
			} {
				assert.Equal(t, held, string(want),
					"the identity is the specification's own spelling")
			}
		})

		t.Run("is distinct for every language declared here", func(t *testing.T) {
			t.Parallel()
			// Two languages sharing one identity open each other's files
			// under the wrong name, and a server checks them by the wrong
			// rules while answering as though it had not.
			seen := map[string]bool{}
			for _, held := range []string{
				lsp.IdentityC, lsp.IdentityCSharp, lsp.IdentityGo, lsp.IdentityJava,
				lsp.IdentityJavaScript, lsp.IdentityPython, lsp.IdentityRuby,
				lsp.IdentityRust, lsp.IdentityScala, lsp.IdentityTypeScript,
			} {
				assert.False(t, seen[held], "no identity is declared twice")
				seen[held] = true
			}
			assert.Length(t, seen, 10, "one for each language techne serves")
		})
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("claims the roles binding answers", func(t *testing.T) {
			t.Parallel()
			for _, role := range []engine.Role{
				engine.RoleResolve, engine.RoleRelate, engine.RolePlan, engine.RoleVerify,
			} {
				assert.Equal(t, lsp.Binding()[role], trust.Resolved,
					"a type checker binds names, which is what these ask about: "+role.String())
			}
		})

		t.Run("leaves outline and search to a parser", func(t *testing.T) {
			t.Parallel()
			// A server outlines one file no better than a parser does at
			// a thousandth of the speed, and its workspace query is
			// answered from an index it caps without saying so, which can
			// never support a claim that something is absent.
			for _, role := range []engine.Role{engine.RoleOutline, engine.RoleSearch} {
				_, claimed := lsp.Binding()[role]
				assert.False(t, claimed,
					"claiming this would make a parser lose a role it does better: "+role.String())
			}
		})

		t.Run("is a copy, so one language cannot alter another's", func(t *testing.T) {
			t.Parallel()
			// Ten modules call this. A shared map would let a language
			// that narrowed its own server silently narrow every other.
			held := lsp.Binding()
			delete(held, engine.RolePlan)
			_, still := lsp.Binding()[engine.RolePlan]
			assert.True(t, still, "what one caller does to its map is its own")
		})

		t.Run("passes the validation a declaration must", func(t *testing.T) {
			t.Parallel()
			held := lsp.Server{
				Name: "any", Command: []string{"any"},
				LanguageID: lsp.IdentityGo, Serves: lsp.Binding(),
			}
			assert.NoError(t, held.Valid(),
				"the shape ten modules share is one a registry accepts")
		})
	})
}
