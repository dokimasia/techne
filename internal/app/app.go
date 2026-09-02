// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/c"
	"go.dokimi.dev/techne/lang/csharp"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/java"
	"go.dokimi.dev/techne/lang/javascript"
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
type register func(fs.FS, *lang.Registry, *engine.Catalog) error

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
}

// Build assembles a server over one workspace.
//
// The workspace is given twice: as an [io/fs.FS] the engines read
// through, and as the [change.Files] the write path writes through.
// Passing nil for the second registers the read tools alone, which is
// what a caller with nothing to write to wants.
//
// It reports an error when a language module refuses to register, which
// is a mistake in that module rather than something a caller did.
func Build(fsys fs.FS, files change.Files) (*Server, error) {
	registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
	for _, add := range languages {
		if err := add(fsys, registry, catalogue); err != nil {
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
		err = errors.Join(err, offer(tool.Document(reads, change.New(catalogue, registry, files))))
	}
	if err != nil {
		return nil, fmt.Errorf("app: %w", err)
	}

	return &Server{Tools: tools, Languages: registry.Languages()}, nil
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

	built, err := Build(workspace.FS(), workspace)
	if err != nil {
		return err
	}

	server, err := presenter.NewServer(built.Tools, presenter.Info{Name: "techne", Version: version})
	if err != nil {
		return fmt.Errorf("app: %w", err)
	}
	return presenter.Serve(ctx, server)
}
