// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
)

// gopls returns a complete declaration of gopls that serves relate.
func gopls() lsp.Server {
	return lsp.Server{
		Name: "gopls", Command: []string{"gopls"}, LanguageID: lsp.IdentityGo,
		Serves: map[engine.Role]trust.Fidelity{engine.RoleRelate: trust.Resolved},
	}
}

func TestServer(t *testing.T) {
	t.Parallel()

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared tier of a role", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, gopls().Fidelity(engine.RoleRelate), trust.Resolved, "the tier of RoleRelate")
		})

		t.Run("returns None for a role without a tier", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, gopls().Fidelity(engine.RoleFormat), trust.None, "the tier of RoleFormat")
		})
	})

	t.Run("Installed", func(t *testing.T) {
		t.Parallel()

		t.Run("names a program that is not on PATH", func(t *testing.T) {
			t.Parallel()
			server := gopls()
			server.Command = []string{"techne-no-such-language-server"}
			err := server.Installed()
			assert.HasError(t, err, "Installed of a missing program")
			assert.Contains(t, err.Error(), "techne-no-such-language-server", "the error of Installed")
			assert.Contains(t, err.Error(), "PATH", "the error of Installed")
		})
	})

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		t.Run("accepts a complete declaration", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, gopls().Valid(), "Valid of a complete declaration")
		})

		t.Run("accepts a declaration with a loading time", func(t *testing.T) {
			t.Parallel()
			server := gopls()
			server.Loading = 90 * time.Second
			assert.NoError(t, server.Valid(), "Valid of a declaration with Loading")
		})

		for _, missing := range []struct {
			field string
			clear func(*lsp.Server)
		}{
			{"a name", func(s *lsp.Server) { s.Name = "" }},
			{"a command", func(s *lsp.Server) { s.Command = nil }},
			{"a language identifier", func(s *lsp.Server) { s.LanguageID = "" }},
			{"a role", func(s *lsp.Server) { s.Serves = nil }},
		} {
			t.Run("refuses a declaration without "+missing.field, func(t *testing.T) {
				t.Parallel()
				server := gopls()
				missing.clear(&server)
				assert.HasError(t, server.Valid(), "Valid of a declaration without "+missing.field)
			})
		}

		t.Run("refuses a role at no tier", func(t *testing.T) {
			t.Parallel()
			server := gopls()
			server.Serves = map[engine.Role]trust.Fidelity{engine.RoleRelate: trust.None}
			assert.HasError(t, server.Valid(), "Valid of a role at trust.None")
		})

		t.Run("refuses a role that the engine does not serve", func(t *testing.T) {
			t.Parallel()
			server := gopls()
			server.Serves = map[engine.Role]trust.Fidelity{engine.RoleOutline: trust.Resolved}
			err := server.Valid()
			assert.HasError(t, err, "Valid of RoleOutline")
			assert.Contains(t, err.Error(), engine.RoleOutline.String(), "the error of Valid")
		})
	})

	t.Run("Named", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the dialect of a mapped extension", func(t *testing.T) {
			t.Parallel()
			server := gopls()
			server.Dialects = map[string]string{".tsx": lsp.IdentityTypeScriptReact}
			assert.Equal(t, server.Named("app/view.tsx"), lsp.IdentityTypeScriptReact, "the identifier of view.tsx")
		})

		t.Run("returns the language identifier of another extension", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, gopls().Named("main.go"), lsp.IdentityGo, "the identifier of main.go")
		})
	})

	t.Run("Offered", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false without a kind or a title", func(t *testing.T) {
			t.Parallel()
			assert.False(t, lsp.Refactor{}.Offered(), "Offered of the zero Refactor")
		})

		t.Run("reports true for a kind", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lsp.Refactor{Kind: "refactor.extract.function"}.Offered(), "Offered of a kind")
		})

		t.Run("reports true for a title", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lsp.Refactor{Titles: []string{"extract method"}}.Offered(), "Offered of a title")
		})
	})
}
