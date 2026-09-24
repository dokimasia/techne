// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// register is the registration of one language module.
type register func(lang.Workspace, *lang.Registry, *engine.Catalog) error

// languages are the registrations of the ten language modules of the binary.
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

// mockVar is the variable of the environment whose value [Run] passes to [Build] as the
// specification of the mock languages.
const mockVar = "TECHNE_MOCK"

// shutting is the time that [Run] gives the engines to stop after the session ends.
const shutting = 5 * time.Second

// Server is the tools and the languages of one workspace.
type Server struct {
	// Tools are the tools that a client can call.
	Tools *tool.Registry

	// Languages are the registered languages.
	Languages []source.Language

	// engines are the engines of the languages, which Close closes.
	engines *engine.Catalog
}

// Close closes every engine of the server that implements [engine.Closer], such as the engine
// of a language server, within ctx. A language server starts at the first call of its engine.
func (s *Server) Close(ctx context.Context) error { return s.engines.Close(ctx) }

// Build returns the server of the workspace w. It registers the ten language modules and the
// mock languages of the specification mocks.
//
// The read tools read w, and the write tools write through files. Build leaves the write tools
// out for a nil files. For a workspace that is not on disk, it registers only the engines that
// read w.FS, such as the parsers, because a language server and the Go checker read the files
// under w.Root.
//
// mocks is a list of entries separated by commas. An entry is the name of a mock language,
// then optionally @ and a word of [trust.Fidelities], then optionally / and a word of
// [trust.Completenesses]. The spec 1 or true is one language named mock, and the empty spec,
// 0 or false is none.
//
// It returns an error for:
//
//   - a word of mocks that neither vocabulary has
//   - a language module that does not register
//   - a tool that does not build
func Build(w lang.Workspace, files change.Files, mocks string) (*Server, error) {
	simulated, err := mocked(mocks)
	if err != nil {
		return nil, err
	}
	registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
	for _, add := range append(slices.Clone(languages), simulated...) {
		if err = add(w, registry, catalogue); err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
	}

	reads := query.New(catalogue, registry)
	tools := tool.NewRegistry()
	offer := func(t tool.Tool, failed error) error {
		if failed != nil {
			return failed
		}
		return tools.Add(t)
	}
	err = errors.Join(
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

// mocked returns the registrations of the mock languages of spec, in the syntax that [Build]
// states.
func mocked(spec string) ([]register, error) {
	spec = strings.TrimSpace(spec)
	switch spec {
	case "", "0", "false":
		return nil, nil
	case "1", "true":
		spec = mock.Language
	}

	var out []register
	for entry := range strings.SplitSeq(spec, ",") {
		if entry = strings.TrimSpace(entry); entry == "" {
			continue
		}
		simulated, err := simulating(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, simulated)
	}
	return out, nil
}

// simulating returns the registration of the mock language of one entry of a specification:
// a name, then optionally @ and a fidelity, then optionally / and a completeness.
func simulating(entry string) (register, error) {
	name, tiers, _ := strings.Cut(entry, "@")
	fidelity, completeness, _ := strings.Cut(tiers, "/")

	var opts []mock.Option
	if fidelity != "" {
		f, err := word(name, "fidelity", fidelity, trust.Fidelities())
		if err != nil {
			return nil, err
		}
		opts = append(opts, mock.At(f))
	}
	if completeness != "" {
		c, err := word(name, "completeness", completeness, trust.Completenesses())
		if err != nil {
			return nil, err
		}
		opts = append(opts, mock.Covering(c))
	}
	return mock.Registering(name, opts...), nil
}

// word returns the value of values whose word is given, and an error that lists the words of
// values when no value has it. name is the mock language, and what is the kind of the word.
func word[T fmt.Stringer](name, what, given string, values []T) (T, error) {
	words := make([]string, 0, len(values))
	for _, v := range values {
		if v.String() == given {
			return v, nil
		}
		words = append(words, v.String())
	}
	var zero T
	return zero, fmt.Errorf("app: the mock language %s does not take the %s %q. It takes %s",
		name, what, given, strings.Join(words, ", "))
}

// Root returns given as an absolute path without symbolic links, and the working directory for
// the empty path. [Run] passes this form to every engine and to the write path, so each of them
// names a file of the workspace by the same path. It returns an error for a path that does not
// exist.
func Root(given string) (string, error) {
	if given == "" {
		given = "."
	}
	absolute, err := filepath.Abs(given)
	if err != nil {
		return "", fmt.Errorf("app: the workspace root %q: %w", given, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("app: the workspace root %q: %w", given, err)
	}
	return resolved, nil
}

// Run serves the workspace at root to a client over standard input and output. [Root] resolves
// root, the variable TECHNE_MOCK is the specification of the mock languages of [Build], and the
// server reports version to the client.
//
// Run returns nil when the client closes standard input, and the error of ctx when ctx is done.
// After the session it gives the engines five seconds to stop.
func Run(ctx context.Context, root, version string) error {
	resolved, err := Root(root)
	if err != nil {
		return err
	}
	workspace, err := files.Open(resolved)
	if err != nil {
		return err
	}
	defer func() { _ = workspace.Close() }()

	built, err := Build(lang.Workspace{FS: workspace.FS(), Root: resolved}, workspace, os.Getenv(mockVar))
	if err != nil {
		return err
	}
	defer func() {
		// ctx is done after a signal, so the engines stop on a context that ctx does not cancel.
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
