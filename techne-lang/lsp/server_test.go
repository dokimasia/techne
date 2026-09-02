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

// declaring is a well-formed server declaration.
func declaring() lsp.Server {
	return lsp.Server{
		Name: "gopls", Command: []string{"gopls"}, LanguageID: "go",
		Serves: map[engine.Role]trust.Fidelity{
			engine.RoleOutline: trust.Resolved,
			engine.RoleRelate:  trust.Resolved,
		},
	}
}

func TestServer(t *testing.T) {
	t.Parallel()

	t.Run("Reaches", func(t *testing.T) {
		t.Parallel()

		t.Run("is what the language declared, per role", func(t *testing.T) {
			t.Parallel()
			// Per role because they are not the same. A server binds
			// names through a type checker and outlines a file no better
			// than a parser does, at a thousandth of the speed.
			assert.Equal(t, declaring().Reaches(engine.RoleRelate), trust.Resolved,
				"a server binds names, which is what it is for")
		})

		t.Run("is nothing for a role it does not serve", func(t *testing.T) {
			t.Parallel()
			// A catalogue sorts by evidence. A server claiming a role it
			// cannot serve wins the sort and answers with nothing.
			assert.Equal(t, declaring().Reaches(engine.RoleFormat), trust.None,
				"a role nobody declared is one this server does not reach")
		})
	})

	t.Run("Installed", func(t *testing.T) {
		t.Parallel()

		t.Run("says what is missing rather than only that something is", func(t *testing.T) {
			t.Parallel()
			// A missing tool is fixed by installing something and a
			// missing capability is not. A caller told only no cannot
			// tell them apart.
			held := declaring()
			held.Command = []string{"a-server-nobody-has-installed"}
			err := held.Installed()

			assert.HasError(t, err, "a server not on the path cannot run")
			assert.Contains(t, err.Error(), "a-server-nobody-has-installed",
				"and the reason names what to install")
			assert.Contains(t, err.Error(), "PATH", "and where it was looked for")
		})
	})

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		t.Run("accepts a declaration that can be run", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, declaring().Valid(), "a complete declaration is usable")
		})

		t.Run("refuses one missing what a server cannot work without", func(t *testing.T) {
			t.Parallel()
			// Checked when a module registers rather than when a call
			// arrives. A server declared with no language id opens every
			// file under an empty name and answers about nothing, which
			// is the hardest failure to notice in a system whose job
			// includes reporting that it found nothing.
			for _, one := range []struct {
				what   string
				broken func(lsp.Server) lsp.Server
			}{
				{"a name", func(s lsp.Server) lsp.Server { s.Name = ""; return s }},
				{"a command", func(s lsp.Server) lsp.Server { s.Command = nil; return s }},
				{"a language id", func(s lsp.Server) lsp.Server { s.LanguageID = ""; return s }},
				{"a role", func(s lsp.Server) lsp.Server { s.Serves = nil; return s }},
			} {
				assert.HasError(t, one.broken(declaring()).Valid(),
					"a declaration without "+one.what+" cannot serve anything")
			}
		})

		t.Run("refuses a role claimed with no evidence behind it", func(t *testing.T) {
			t.Parallel()
			held := declaring()
			held.Serves = map[engine.Role]trust.Fidelity{engine.RoleOutline: trust.None}
			assert.HasError(t, held.Valid(),
				"a role claimed at no tier would be selected for and answer for nothing")
		})
	})
}

func TestLoading(t *testing.T) {
	t.Parallel()

	t.Run("Extracts", func(t *testing.T) {
		t.Parallel()

		t.Run("says whether the server offers the refactoring at all", func(t *testing.T) {
			t.Parallel()
			// A server nobody declared this for is not one that refuses
			// it: it is one nobody has asked. pyright, clangd and metals
			// offer nothing over a run of statements, and that is a fact
			// about each of them rather than a default.
			assert.False(t, declaring().Extracts.Offered(),
				"a declaration that names no action offers none")

			held := declaring()
			held.Extracts = lsp.Refactor{Kind: "refactor.extract.function"}
			assert.True(t, held.Extracts.Offered(), "a kind alone is enough to ask for one")

			held.Extracts = lsp.Refactor{Titles: []string{"extract method"}}
			assert.True(t, held.Extracts.Offered(),
				"and so is a wording, for a server that sets no kind")
		})
	})

	t.Run("Loading", func(t *testing.T) {
		t.Parallel()

		t.Run("is declared per server, because they differ by orders", func(t *testing.T) {
			t.Parallel()
			// A parser-backed server is ready in milliseconds and one
			// that imports a build system is not ready for minutes. One
			// figure for both either stalls every question or reports
			// every slow server's answers as short.
			held := pretending(modeDefault)
			held.Loading = 90 * time.Second

			assert.NoError(t, held.Valid(), "a declared wait is part of a sound declaration")
			assert.Equal(t, held.Loading, 90*time.Second, "and is kept as declared")
		})

		t.Run("is optional, and zero takes the default", func(t *testing.T) {
			t.Parallel()
			held := pretending(modeDefault)
			held.Loading = 0
			assert.NoError(t, held.Valid(),
				"a server with nothing to say about its own start is still valid")
		})
	})
}
