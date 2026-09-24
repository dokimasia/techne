// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// TestMain runs the scripted server of lsptest when a test starts this binary as one, and the
// tests otherwise.
func TestMain(m *testing.M) { lsptest.Main(m) }

// serving returns an engine over a new workspace of files that runs the scripted server in
// mode.
func serving(t *testing.T, mode lsptest.Mode, files map[string]string, options ...lsptest.Option) *lsp.Engine {
	t.Helper()
	return lsptest.Engine(t, lsptest.Workspace(t, files), lsptest.Server(mode, options...))
}

// sample returns a workspace with [lsptest.Content] in a.fake.
func sample() map[string]string { return map[string]string{"a.fake": lsptest.Content} }

// declared returns the identity of the declaration name of kind in a.fake.
func declared(name string, kind sema.Kind) sema.ID {
	return sema.NewID(lsptest.Language, ".", name, kind)
}

// store is the position of the name Store in [lsptest.Content]: line 2, column 5.
func store() source.Position { return source.Position{Line: 2, Column: 5} }

// missing returns a declaration of a server whose program is not on PATH.
func missing() lsp.Server {
	server := lsptest.Server(lsptest.Default)
	server.Command = []string{"techne-no-such-language-server"}
	return server
}

// hasCaveat reports whether caveats contain a caveat with code.
func hasCaveat(caveats []trust.Caveat, code trust.CaveatCode) bool {
	return slices.ContainsFunc(caveats, func(one trust.Caveat) bool { return one.Code == code })
}

// messages returns the diagnostic messages of findings, in order.
func messages(findings []edit.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, one := range findings {
		out = append(out, one.Diagnostic.Message)
	}
	return out
}

// names returns the names of symbols, in order.
func names(symbols []sema.Symbol) []string {
	out := make([]string, 0, len(symbols))
	for _, one := range symbols {
		out = append(out, one.Name)
	}
	return out
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration without a language", func(t *testing.T) {
			t.Parallel()
			_, err := lsp.New(t.TempDir(), lang.Declaration{}, lsptest.Server(lsptest.Default), nil)
			assert.HasError(t, err, "New returns an error for an empty lang.Declaration")
		})

		t.Run("refuses a root that is a file", func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(lsptest.Workspace(t, sample()), "a.fake")
			_, err := lsp.New(file, lsptest.Declaration(), lsptest.Server(lsptest.Default), nil)
			assert.HasError(t, err, "New returns an error for the root "+file)
		})

		t.Run("refuses a server declaration without a language identifier", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Default)
			server.LanguageID = ""
			_, err := lsp.New(t.TempDir(), lsptest.Declaration(), server, nil)
			assert.HasError(t, err, "New returns the error of Server.Valid")
		})

		t.Run("maps a path the server resolves through a symbolic link into the workspace", func(t *testing.T) {
			t.Parallel()
			real := lsptest.Workspace(t, sample())
			link := filepath.Join(t.TempDir(), "link")
			assert.NoError(t, os.Symlink(real, link), "the test links "+link+" to "+real)
			e := lsptest.Engine(t, link, lsptest.Server(lsptest.Canonical))

			got, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve through the linked root")
			assert.Length(t, got.Items, 1, "the declarations that Store denotes")
			assert.Equal(t, got.Items[0].Span.Path, source.Path("a.fake"),
				"the path of Store, relative to the workspace")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of the server", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			assert.Equal(t, e.Name(), lsptest.Name, "the Name of the engine")
		})
	})

	t.Run("Language", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared language", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			assert.Equal(t, e.Language(), lsptest.Language, "the Language of the engine")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared tier of a role", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			assert.Equal(t, e.Fidelity(engine.RoleResolve), trust.Resolved, "the fidelity of RoleResolve")
		})

		t.Run("returns None for a role without a declared tier", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			assert.Equal(t, e.Fidelity(engine.RoleFormat), trust.None, "the fidelity of RoleFormat")
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("returns CostSession for every role", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			for _, role := range engine.Roles() {
				assert.Equal(t, e.Cost(role), engine.CostSession, "the cost of "+role.String())
			}
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("names a program that is not on PATH", func(t *testing.T) {
			t.Parallel()
			e, err := lsp.New(t.TempDir(), lsptest.Declaration(), missing(), nil)
			assert.NoError(t, err, "New accepts a server that is not installed")
			err = e.Available(t.Context())
			assert.HasError(t, err, "Available for a missing program")
			assert.Contains(t, err.Error(), "techne-no-such-language-server", "the error of Available")
		})

		t.Run("returns nil for a program on PATH", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			assert.NoError(t, e.Available(t.Context()), "Available for the test binary")
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for an engine without a server", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			assert.NoError(t, e.Close(t.Context()), "Close before any question")
		})

		t.Run("returns nil for a second call", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve starts the server")
			assert.NoError(t, e.Close(t.Context()), "the first Close")
			assert.NoError(t, e.Close(t.Context()), "the second Close")
		})

		t.Run("lets the next question start a new server", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Default, lsptest.RecordStarts(log)))

			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "the first Resolve")
			assert.NoError(t, e.Close(t.Context()), "Close after the first Resolve")
			_, err = e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "the Resolve after Close")
			assert.Equal(t, starts(t, log), 2, "the number of servers started")
		})

		t.Run("stops the server while questions run", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve starts the server")

			var group sync.WaitGroup
			for range 4 {
				group.Go(func() {
					for range 10 {
						_, _ = e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
					}
				})
			}
			var closed error
			group.Go(func() { closed = e.Close(t.Context()) })
			group.Wait()
			if closed != nil && !strings.Contains(closed.Error(), "shutdown") {
				t.Errorf("Close returned %v, want nil or a shutdown error", closed)
			}
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("sends the content of a file changed on disk", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": "first\nrest\n"})
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Echoes))
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "the first Verify")

			rewrite(t, root, "a.fake", "second\nrest\n")
			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "the second Verify")
			assert.Contains(t, messages(got.Items), "first=second", "the first line of the buffer")
			assert.Contains(t, messages(got.Items), "version=2", "the version of the buffer")
		})

		t.Run("sends no change for a file rewritten with its content", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": "first\nrest\n"})
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Echoes))
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "the first Verify")

			rewrite(t, root, "a.fake", "first\nrest\n")
			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "the second Verify")
			assert.Contains(t, messages(got.Items), "version=1", "the version of the buffer")
		})

		t.Run("releases the buffer of a deleted file", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": "first\n", "b.fake": "second\n"})
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Echoes))
			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of the workspace")
			assert.Contains(t, messages(got.Items), "holding=a.fake,b.fake", "the buffers after the first Verify")

			assert.NoError(t, os.Remove(filepath.Join(root, "b.fake")), "the test deletes b.fake")
			got, err = e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify of a.fake")
			assert.Contains(t, messages(got.Items), "holding=a.fake", "the buffers after b.fake is gone")
		})

		t.Run("reads no file whose size and modification time are unchanged", func(t *testing.T) {
			t.Parallel()
			if os.Geteuid() == 0 {
				t.Skip("root reads a file without read permission")
			}
			root := lsptest.Workspace(t, map[string]string{"a.fake": "first\n", "b.fake": "second\n"})
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Echoes))
			for _, scope := range []source.Path{"a.fake", "b.fake"} {
				_, err := e.Verify(t.Context(), engine.Request{Scope: scope}, nil)
				assert.NoError(t, err, "Verify of "+string(scope))
			}

			assert.NoError(t, os.Chmod(filepath.Join(root, "a.fake"), 0), "the test makes a.fake unreadable")
			got, err := e.Verify(t.Context(), engine.Request{Scope: "b.fake"}, nil)
			assert.NoError(t, err, "Verify of b.fake")
			assert.Contains(t, messages(got.Items), "holding=a.fake,b.fake", "the buffers after a.fake is unreadable")
		})
	})

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("tells the server that a file changed on disk", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, sample())
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Watches))
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify opens a.fake")

			rewrite(t, root, "a.fake", lsptest.Content+"\n")
			_, err = e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of a rename after a.fake changed on disk")
		})

		t.Run("sends a changed file that the question does not name", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content, "b.fake": "nothing\n"})
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Opened))
			for _, scope := range []source.Path{"a.fake", "b.fake"} {
				_, err := e.Resolve(t.Context(), engine.Request{Scope: scope}, store())
				assert.NoError(t, err, "Resolve opens "+string(scope))
			}

			rewrite(t, root, "b.fake", "var _ Store\n")
			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of a rename after b.fake changed on disk")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake", "b.fake"}, "the files the rename changes")
		})
	})
}

// rewrite writes content to the file name under root.
func rewrite(t *testing.T, root, name, content string) {
	t.Helper()
	assert.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644),
		"the test writes "+name)
}

// starts returns how many times the scripted server started, from the file of
// [lsptest.RecordStarts].
func starts(t *testing.T, log string) int {
	t.Helper()
	recorded, err := os.ReadFile(log)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	assert.NoError(t, err, "the test reads "+log)
	return strings.Count(string(recorded), "\n")
}

// paths returns the path of each change, in order.
func paths(changes []edit.Change) []source.Path {
	out := make([]source.Path, 0, len(changes))
	for _, one := range changes {
		out = append(out, one.Path)
	}
	return out
}
