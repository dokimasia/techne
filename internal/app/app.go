// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/c"
	"go.dokimi.dev/techne/lang/csharp"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/java"
	"go.dokimi.dev/techne/lang/javascript"
	"go.dokimi.dev/techne/lang/mock"
	"go.dokimi.dev/techne/lang/python"
	"go.dokimi.dev/techne/lang/ruby"
	"go.dokimi.dev/techne/lang/rust"
	"go.dokimi.dev/techne/lang/scala"
	"go.dokimi.dev/techne/lang/typescript"
	"go.dokimi.dev/techne/presenter"
	"go.dokimi.dev/techne/service/change"
	"go.dokimi.dev/techne/service/query"
	"go.dokimi.dev/techne/service/workspace/files"
	"go.dokimi.dev/techne/tool"
)

// register is one language module's entry point. The list below is the
// only place techne names a language.
type register func(lang.Workspace, *lang.Registry, *engine.Catalog) error

// languages are the modules this binary ships with.
//
// Removing a language takes four edits, all in the root module: this
// list, the import above, the require and replace in go.mod, and the use
// line in go.work. The directory then goes. No other module names a
// language, so none of them is touched.
var languages = []register{
	c.Register,
	csharp.Register,
	golang.Register,
	java.Register,
	javascript.Register,
	python.Register,
	ruby.Register,
	rust.Register,
	scala.Register,
	typescript.Register,
}

// Server is everything a transport needs, and what a test inspects.
type Server struct {
	// Tools is what an agent may call.
	Tools *tool.Registry
	// Languages are the ones registered, so a caller can report what
	// this binary serves without knowing what was compiled in.
	Languages []source.Language

	// engines is what has to be stopped on the way out. A language
	// server is a process, and one left running per language per run is
	// a leak nobody sees until the machine is out of memory.
	engines *engine.Catalog
}

// Close stops every engine holding something that outlives a call.
//
// Whoever built the server calls it. An engine nobody asked anything
// started nothing, so closing a server that answered no questions does
// nothing.
func (s *Server) Close(ctx context.Context) error { return s.engines.Close(ctx) }

// mocked is the mock languages to register, read from the environment.
//
// Off unless asked for. A fake language in a real server's tool surface
// would have an agent routing real work to something that answers from a
// toy grammar, so it takes an explicit word to turn on.
//
//	TECHNE_MOCK=1                      one language, called mock
//	TECHNE_MOCK=alpha,beta             two, which route separately
//	TECHNE_MOCK=alpha,beta@syntactic   one that resolves beside one that
//	                                   only parses
//	TECHNE_MOCK=alpha@resolved/partial one that binds names and has not
//	                                   finished reading the workspace
//
// The tiers are what make a refusal drivable rather than described: an
// operation that rewrites references is refused on partial coverage
// however strong the binding, and there is no other way to stand a
// workspace up in that state.
func mocked() []register {
	held := strings.TrimSpace(os.Getenv(mockVar))
	switch held {
	case "", "0", "false":
		return nil
	case "1", "true":
		held = mock.Language
	}

	var out []register
	for spec := range strings.SplitSeq(held, ",") {
		if spec = strings.TrimSpace(spec); spec != "" {
			out = append(out, simulating(spec))
		}
	}
	return out
}

// simulating reads one name@fidelity/completeness and returns its
// registration.
//
// A tier nobody recognises is left at the default rather than refused.
// This is a switch for driving the tools by hand, and failing to start
// over a typo in it would be the wrong trade.
func simulating(spec string) register {
	name, tiers, _ := strings.Cut(spec, "@")
	bound, covered, _ := strings.Cut(tiers, "/")

	var opts []mock.Option
	for held, f := range map[string]trust.Fidelity{
		"none": trust.None, "syntactic": trust.Syntactic,
		"indexed": trust.Indexed, "resolved": trust.Resolved,
	} {
		if held == bound {
			opts = append(opts, mock.At(f))
		}
	}
	for held, c := range map[string]trust.Completeness{
		"unknown": trust.ScopeUnknown, "partial": trust.ScopePartial,
		"total": trust.ScopeTotal,
	} {
		if held == covered {
			opts = append(opts, mock.Covering(c))
		}
	}
	return mock.Registering(name, opts...)
}

// mockVar names the languages to simulate.
const mockVar = "TECHNE_MOCK"

// shutting is how long a language server is given to stop before it is
// killed. Long enough for one to write out what it was holding, short
// enough that a client closing a connection does not wait on it.
const shutting = 5 * time.Second

// Build assembles a server over one workspace.
//
// The workspace is given twice: as the tree the engines read, and as the
// [change.Files] the write path writes through. Passing nil for the
// second registers the read tools alone, which is what a caller with
// nothing to write to wants.
//
// A workspace carrying no root on disk registers parsers alone. A
// language server is a process that opens files by name and cannot be
// pointed at a tree that is nowhere, so it is left out rather than
// declared and then failing on the first call.
//
// It reports an error when a language module refuses to register, which
// is a mistake in that module rather than something a caller did.
//
// The result holds processes once anything is asked of it, so a caller
// closes it.
func Build(w lang.Workspace, files change.Files) (*Server, error) {
	registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
	shipped := append(slices.Clone(languages), mocked()...)
	for _, add := range shipped {
		if err := add(w, registry, catalogue); err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
	}

	reads := query.New(catalogue, registry)
	tools := tool.NewRegistry()

	// Every tool is built the same way and fails the same way, so the
	// list below reads as what this server offers rather than as a
	// check after each one. Joining rather than returning at the first
	// means one run reports every tool that is broken.
	offer := func(t tool.Tool, err error) error {
		if err != nil {
			return err
		}
		return tools.Add(t)
	}
	err := errors.Join(
		offer(tool.Outline(reads)),
		offer(tool.Search(reads)),
		offer(tool.Resolve(reads)),
		offer(tool.Relations(reads, reads)),
		offer(tool.Verify(reads)),
		offer(tool.Capabilities(catalogue)),
	)
	if files != nil {
		writes := change.New(catalogue, registry, files)
		err = errors.Join(err,
			offer(tool.Document(reads, writes)),
			offer(tool.Rename(reads, writes)),
			offer(tool.Move(writes)),
			offer(tool.Extract(writes)),
			offer(tool.Apply(writes)),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("app: %w", err)
	}

	return &Server{Tools: tools, Languages: registry.Languages(), engines: catalogue}, nil
}

// Run serves one workspace over stdio until the context is cancelled or
// the client disconnects.
func Run(ctx context.Context, root, version string) error {
	if root == "" {
		working, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("app: workspace root: %w", err)
		}
		root = working
	}

	workspace, err := files.Open(root)
	if err != nil {
		return err
	}
	defer func() { _ = workspace.Close() }()

	built, err := Build(lang.Workspace{FS: workspace.FS(), Root: root}, workspace)
	if err != nil {
		return err
	}
	// The servers outlive the request that started them and are stopped
	// here, on a context of its own: the one that served is cancelled by
	// the time this runs, and a shutdown needs one that is not.
	defer func() {
		stopping, stop := context.WithTimeout(context.WithoutCancel(ctx), shutting)
		defer stop()
		_ = built.Close(stopping)
	}()

	server, err := presenter.NewServer(built.Tools, presenter.Info{Name: "techne", Version: version})
	if err != nil {
		return fmt.Errorf("app: %w", err)
	}
	return presenter.Serve(ctx, server)
}
