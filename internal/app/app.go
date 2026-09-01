// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/query"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/tool"
	"go.dokimi.dev/techne/lang"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/java"
	"go.dokimi.dev/techne/lang/python"
	"go.dokimi.dev/techne/lang/rust"
	"go.dokimi.dev/techne/lang/typescript"
	"go.dokimi.dev/techne/presenter"
)

// register is one language module's entry point. The list below is the
// only place techne names a language.
type register func(fs.FS, *lang.Registry, *engine.Catalog) error

// languages are the modules this binary ships with.
//
// Removing one from this list removes it from the binary; removing its
// directory and its go.work line removes it from the tree.
var languages = []register{
	golang.Register,
	python.Register,
	java.Register,
	rust.Register,
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
// It reports an error when a language module refuses to register, which
// is a mistake in that module rather than something a caller did.
func Build(fsys fs.FS) (*Server, error) {
	registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
	for _, add := range languages {
		if err := add(fsys, registry, catalogue); err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
	}

	reads := query.New(catalogue, registry)

	outline, err := tool.Outline(reads)
	if err != nil {
		return nil, fmt.Errorf("app: %w", err)
	}
	search, err := tool.Search(reads)
	if err != nil {
		return nil, fmt.Errorf("app: %w", err)
	}
	capabilities, err := tool.Capabilities(catalogue)
	if err != nil {
		return nil, fmt.Errorf("app: %w", err)
	}

	tools := tool.NewRegistry()
	for _, t := range []tool.Tool{outline, search, capabilities} {
		if err := tools.Add(t); err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
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

	built, err := Build(os.DirFS(root))
	if err != nil {
		return err
	}

	server, err := presenter.NewServer(built.Tools, presenter.Info{Name: "techne", Version: version})
	if err != nil {
		return fmt.Errorf("app: %w", err)
	}
	return presenter.Serve(ctx, server)
}
