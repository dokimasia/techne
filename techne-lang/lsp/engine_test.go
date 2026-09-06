// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
)

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration that names no language", func(t *testing.T) {
			t.Parallel()
			_, err := lsp.New(t.TempDir(), lang.Declaration{}, pretending(modeDefault))
			assert.HasError(t, err, "an engine answers about one language and must know which")
		})

		t.Run("refuses a root that is not a directory", func(t *testing.T) {
			t.Parallel()
			// A language server is a process that opens files itself and
			// cannot be handed a tree that is not on disk. Refused here
			// rather than on the first call.
			held := filepath.Join(workspace(t, map[string]string{"a.fake": content}), "a.fake")

			_, err := lsp.New(held, declared(), pretending(modeDefault))
			assert.HasError(t, err, "a file is not a workspace")
		})

		t.Run("refuses a server declaration that cannot be run", func(t *testing.T) {
			t.Parallel()
			// Checked when a module registers rather than when a call
			// arrives: a server with no language id opens every file
			// under an empty name and is answered about nothing.
			held := pretending(modeDefault)
			held.LanguageID = ""

			_, err := lsp.New(t.TempDir(), declared(), held)
			assert.HasError(t, err, "a declaration missing what a server needs is refused")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("is the server rather than the language", func(t *testing.T) {
			t.Parallel()
			// Told gopls answered, a caller knows what to install, what
			// to upgrade and whose release notes to read. Told "go", it
			// knows none of that.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			assert.Equal(t, e.Name(), "fake", "the server's own name")
			assert.Equal(t, string(e.Language()), "fake", "beside the language it answers about")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is what the language module declared, per role", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			assert.Equal(t, e.Fidelity(engine.RoleResolve), trust.Resolved,
				"the tier the declaration claimed")
		})

		t.Run("is nothing for a role the declaration left out", func(t *testing.T) {
			t.Parallel()
			// A server claiming a tier for a role nobody declared would
			// win a catalogue's sort for work it cannot do.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			assert.Equal(t, e.Fidelity(engine.RoleFormat), trust.None,
				"an undeclared role reaches nothing")
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("is a session rather than what the first call costs", func(t *testing.T) {
			t.Parallel()
			// Priced at the first call, a catalogue would prefer a parser
			// for every question, including the ones only a server can
			// answer.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			assert.Equal(t, e.Cost(engine.RoleResolve), engine.CostSession,
				"dear once and cheap after")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("says what is missing rather than only that something is", func(t *testing.T) {
			t.Parallel()
			// A missing tool is a problem a caller can act on and a
			// missing capability is one to route around. Told only "no",
			// the two look the same.
			e, err := lsp.New(t.TempDir(), declared(), missing())
			assert.NoError(t, err, "a declaration for a server that is not here is still valid")

			err = e.Available(t.Context())
			assert.HasError(t, err, "the server is not on this machine")
			assert.Contains(t, err.Error(), "PATH", "and the reason names what to fix")
		})

		t.Run("passes for a server that is here", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			assert.NoError(t, e.Available(t.Context()), "the fake is this test binary")
		})
	})

	t.Run("a server that does not answer the handshake", func(t *testing.T) {
		t.Parallel()

		t.Run("is given up on rather than waited out", func(t *testing.T) {
			t.Parallel()
			// The connection came up, so the program is there and it is
			// not talking. Measured against typescript-language-server
			// over a repository whose TypeScript version it refuses,
			// fourteen seconds of an agent's turn went on a server that
			// was never going to answer.
			held := pretending(modeSilent)
			e, err := lsp.New(t.TempDir(), declared(), held)
			assert.NoError(t, err, "an engine builds over the workspace")
			stopping(t, e)

			began := time.Now()
			_, err = e.Outline(t.Context(), engine.Request{Scope: "."})
			assert.HasError(t, err, "a server that says nothing answers nothing")
			assert.True(t, time.Since(began) < time.Minute,
				"and is given up on inside a window this engine chose")
		})
	})

	t.Run("the roles it serves", func(t *testing.T) {
		t.Parallel()

		t.Run("are the ones with a request behind them", func(t *testing.T) {
			t.Parallel()
			// A role is declined by lacking a method rather than by
			// returning an error, so what this engine satisfies is the
			// whole of what a catalogue can select it for.
			var held any = serving(t, modeDefault, map[string]string{"a.fake": content})

			for _, one := range []struct {
				role   engine.Role
				serves bool
			}{
				{engine.RoleOutline, true},
				{engine.RoleSearch, true},
				{engine.RoleResolve, true},
				{engine.RoleRelate, true},
				{engine.RolePlan, true},
				{engine.RoleVerify, true},
				{engine.RoleFormat, true},
				{engine.RoleCheck, true},
				{engine.RoleIndex, false},
			} {
				assert.Equal(t, satisfies(held, one.role), one.serves,
					"the port is present exactly where the protocol has a request: "+
						one.role.String())
			}
		})
	})
}

// A server answers about the buffer it was given, not about the file. It
// holds that buffer for the life of the session, and techne's own write
// path rewrites files underneath it: a rename applied in one call leaves
// every later call in the same session answering about text that is no
// longer there.
//
// The failure is silent. A second rename computed against the old text
// finds the uses the old text had, rewrites the declaration and leaves
// the rest — one file changed, a reference in another left pointing at a
// name nothing declares. Driving two renames through one session over a
// real workspace produced exactly that.
func TestEngineOpen(t *testing.T) {
	t.Parallel()

	t.Run("a file that changed since it was opened", func(t *testing.T) {
		t.Parallel()

		t.Run("is sent again, so the server answers about what is there", func(t *testing.T) {
			t.Parallel()
			// The echoing server reports the first line of the buffer it
			// holds as the declaration the file makes, which is the only
			// way to see from outside what a server thinks a file says.
			root := workspace(t, map[string]string{"a.fake": "first\nrest\n"})
			e, err := lsp.New(root, declared(), pretending(modeEchoes))
			assert.NoError(t, err, "an engine builds over the workspace")
			stopping(t, e)

			got, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the first question opens the file")
			assert.Equal(t, names(got.Items), []string{"first"},
				"and the server is holding what was on disk")

			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "a.fake"), []byte("second\nrest\n"), 0o644),
				"something rewrites the file, as the write path does")

			got, err = e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "asking again succeeds")
			assert.Equal(t, names(got.Items), []string{"second"},
				"and the server was told, rather than left holding the old text")
		})

		t.Run("is reported as changed on disk, not only in the buffer", func(t *testing.T) {
			t.Parallel()
			// A server that keeps its own model of the workspace checks
			// it against the filesystem before it refactors and refuses
			// while the two differ. jdtls answers a rename over a file
			// techne's write path rewrote with "out of sync with file
			// system", and a change to the open buffer does not settle
			// it: the file is a different thing from the buffer.
			root := workspace(t, map[string]string{"a.fake": content})
			e, err := lsp.New(root, declared(), pretending(modeWatches))
			assert.NoError(t, err, "an engine builds over the workspace")
			stopping(t, e)

			_, err = e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the first question opens the file")
			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "a.fake"), []byte(content+"\n"), 0o644),
				"the write path rewrites the file")

			_, err = e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{
					Path: "a.fake", Start: source.Position{Offset: 16},
				}},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err,
				"the server was told the file changed on disk, so it will refactor")
		})

		t.Run("is sent again even when the question is about another file", func(t *testing.T) {
			t.Parallel()
			// A server answers from every buffer it holds, not only the
			// one a question names: a rename asks who uses a
			// declaration, and the uses are in other files. Refreshing
			// only the file being asked about left a second rename
			// finding no uses at all, because the file holding them
			// still said what it said before the first.
			root := workspace(t, map[string]string{
				"a.fake": content, "b.fake": "nothing here\n",
			})
			e, err := lsp.New(root, declared(), pretending(modeOpened))
			assert.NoError(t, err, "an engine builds over the workspace")
			stopping(t, e)

			_, err = e.Outline(t.Context(), engine.Request{Scope: "."})
			assert.NoError(t, err, "the first question opens both files")
			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "b.fake"), []byte(content), 0o644),
				"and something rewrites the one the next question is not about")

			got, err := renaming(t, e)
			assert.NoError(t, err, "renaming succeeds")
			assert.Length(t, got.Items, 2,
				"the server was told about the file it was not asked about")
		})

		t.Run("is let go where it is gone", func(t *testing.T) {
			t.Parallel()
			// A move takes a file away. A server left holding the buffer
			// keeps answering about a file that is not there and keeps
			// reporting diagnostics against it.
			root := workspace(t, map[string]string{"a.fake": content, "b.fake": content})
			e, err := lsp.New(root, declared(), pretending(modeEchoes))
			assert.NoError(t, err, "an engine builds over the workspace")
			stopping(t, e)

			_, err = e.Outline(t.Context(), engine.Request{Scope: "."})
			assert.NoError(t, err, "the first question opens both files")

			got, err := e.Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Private: true})
			assert.NoError(t, err, "asking what the server holds succeeds")
			assert.Equal(t, names(got.Items), []string{"a", "b"},
				"which is both of them")

			assert.NoError(t, os.Remove(filepath.Join(root, "b.fake")),
				"and then one of them goes")
			_, err = e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the next question succeeds")

			got, err = e.Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Private: true})
			assert.NoError(t, err, "asking again succeeds")
			assert.Equal(t, names(got.Items), []string{"a"},
				"and the server was told to let the other one go")
		})

		t.Run("is left alone where it did not change", func(t *testing.T) {
			t.Parallel()
			// Re-sending a buffer a server already holds is a version
			// conflict rather than a refresh, which is why this opened
			// once to begin with. What decides is the text, so a file
			// rewritten to what it already said is not sent again.
			root := workspace(t, map[string]string{"a.fake": "first\nrest\n"})
			e, err := lsp.New(root, declared(), pretending(modeEchoes))
			assert.NoError(t, err, "an engine builds over the workspace")
			stopping(t, e)

			_, err = e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the first question opens the file")
			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "a.fake"), []byte("first\nrest\n"), 0o644),
				"the file is written with the same content it held")

			got, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "asking again succeeds")
			assert.Equal(t, names(got.Items), []string{"first"},
				"the server is still holding the one buffer it was given")
		})
	})
}

// satisfies reports whether an engine implements the port for a role.
func satisfies(held any, role engine.Role) bool {
	switch role {
	case engine.RoleOutline:
		_, ok := held.(engine.Outliner)
		return ok
	case engine.RoleSearch:
		_, ok := held.(engine.Searcher)
		return ok
	case engine.RoleResolve:
		_, ok := held.(engine.Resolver)
		return ok
	case engine.RoleRelate:
		_, ok := held.(engine.Relator)
		return ok
	case engine.RolePlan:
		_, ok := held.(engine.Planner)
		return ok
	case engine.RoleVerify:
		_, ok := held.(engine.Verifier)
		return ok
	case engine.RoleFormat:
		_, ok := held.(engine.Formatter)
		return ok
	case engine.RoleCheck:
		_, ok := held.(engine.Checker)
		return ok
	case engine.RoleIndex:
		_, ok := held.(engine.Indexer)
		return ok
	}
	return false
}
