// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest

import (
	"context"
	"os"
	"testing"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
)

// Name is the name of the scripted server.
const Name = "fake"

// The environment variables that pass a declaration to the scripted server.
const (
	envActing   = "TECHNE_LSPTEST_ACTING"
	envMode     = "TECHNE_LSPTEST_MODE"
	envStarts   = "TECHNE_LSPTEST_STARTS"
	envRequests = "TECHNE_LSPTEST_REQUESTS"
	envOutside  = "TECHNE_LSPTEST_OUTSIDE"
	envRenames  = "TECHNE_LSPTEST_RENAMES"
)

// closing is how long [Engine] waits for the scripted server to shut down when a test ends.
const closing = 5 * time.Second

// Main runs the scripted server and exits when the process was started by a declaration
// from [Server]. Otherwise it runs the tests and exits with their status. A test package
// calls it from TestMain.
func Main(m *testing.M) {
	if os.Getenv(envActing) == "" {
		os.Exit(m.Run())
	}
	os.Exit(serve(Mode(os.Getenv(envMode))))
}

// Option changes one answer of the scripted server.
type Option func(*lsp.Server)

// RecordStarts makes the scripted server append one line to the file at path each time it
// starts.
func RecordStarts(path string) Option {
	return func(s *lsp.Server) { s.Env[envStarts] = path }
}

// RecordRequests makes the scripted server append the method of each request it receives to
// the file at path, one line per request.
func RecordRequests(path string) Option {
	return func(s *lsp.Server) { s.Env[envRequests] = path }
}

// Outside makes the scripted server name the file at the absolute path in its answers to
// textDocument/references, textDocument/rename and callHierarchy/outgoingCalls, in place of the
// file the request names.
func Outside(path string) Option {
	return func(s *lsp.Server) { s.Env[envOutside] = path }
}

// Renames makes the scripted server respond to textDocument/rename with edit, a workspace edit
// written as JSON. The server replaces each {root} in edit with the URI of the workspace and
// each {file} with the URI of the document that the request names.
func Renames(edit string) Option {
	return func(s *lsp.Server) { s.Env[envRenames] = edit }
}

// Server returns a declaration that runs the current test binary as the scripted server in
// mode. The declaration claims [trust.Resolved] for resolve, relate, plan, check and verify,
// and no tier for format. The Asks mode is declared with settings, the Loading and Stuck
// modes with a loading time, and the Extracts and Commands modes with the extraction they
// offer.
func Server(mode Mode, options ...Option) lsp.Server {
	server := lsp.Server{
		Name:       Name,
		Command:    []string{os.Args[0]},
		LanguageID: string(Language),
		Env:        map[string]string{envActing: "1", envMode: string(mode)},
		Serves: map[engine.Role]trust.Fidelity{
			engine.RoleResolve: trust.Resolved,
			engine.RoleRelate:  trust.Resolved,
			engine.RolePlan:    trust.Resolved,
			engine.RoleCheck:   trust.Resolved,
			engine.RoleVerify:  trust.Resolved,
		},
	}
	switch mode {
	case Asks:
		server.Settings = map[string]any{configured: map[string]any{"strict": true}}
	case Loading:
		server.Loading = 3 * LoadTime
	case Stuck:
		server.Loading = time.Second
	case Extracts, Commands:
		server.Extracts = lsp.Refactor{Kind: extractKind, Titles: []string{"into function"}}
	}
	for _, option := range options {
		option(&server)
	}
	return server
}

// Engine returns an engine over root that runs server for [Language], and closes the engine
// when the test ends. The test fails if the engine cannot be built.
func Engine(tb testing.TB, root string, server lsp.Server) *lsp.Engine {
	tb.Helper()
	e, err := lsp.New(root, Declaration(), server)
	if err != nil {
		tb.Fatalf("lsptest: build an engine over %s: %v", root, err)
	}
	Cleanup(tb, e)
	return e
}

// Cleanup closes e when the test ends, with a context that the end of the test does not
// cancel and a deadline of 5 seconds. A failed close is logged.
func Cleanup(tb testing.TB, e *lsp.Engine) {
	tb.Helper()
	tb.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.WithoutCancel(tb.Context()), closing)
		defer stop()
		if err := e.Close(ctx); err != nil {
			tb.Logf("lsptest: close the engine of %s: %v", e.Name(), err)
		}
	})
}
