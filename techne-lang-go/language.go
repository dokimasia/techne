// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	_ "embed"
	"fmt"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-go/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/go/checker"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a Go
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "go"

// upstreamQuery is the tags query of the grammar's repository, vendored
// unchanged, so an upgrade of the grammar replaces the file and a review
// reads the diff.
//
//go:embed queries/upstream.scm
var upstreamQuery string

// extendsQuery holds the patterns of the module, which Grammar appends to
// upstreamQuery.
//
//go:embed queries/extends.scm
var extendsQuery string

// Declaration returns the declaration of Go.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".go"},
		Manifests:  []string{"go.mod", "go.work"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "//"},
				{Open: "/*", Close: "*/"},
			},
		},
		Blank:      map[string]bool{"_": true},
		IsTest:     IsTest,
		Namespace:  Unit,
		Visibility: Visibility,
	}
}

// Grammar returns the tree-sitter grammar of Go with the upstream tags
// query and the patterns of the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// The program of gopls, which is also the name of the server, and the
// subcommand that speaks LSP over stdio.
const (
	server = "gopls"
	serves = "serve"
)

// Server returns the declaration of gopls, the language server of the Go
// team. gopls offers to extract a function and a method, each with its own
// kind of code action, so the declaration selects the function by its
// kind.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, serves},
		LanguageID: lsp.IdentityGo,
		Serves:     lsp.Binding(),
		Extracts:   lsp.Refactor{Kind: "refactor.extract.function"},
	}
}

// Register adds Go to r and its engines to c: the tree-sitter engine, and
// for a workspace on disk gopls and the type checker of package checker.
// The type checker serves the roles that need types on a machine without
// gopls, because it runs the go command that builds the workspace.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	checked, err := checking(w)
	if err != nil {
		return err
	}
	return engines.Register(w, r, c, Declaration(), Grammar(), Server(), checked...)
}

// checking returns the type checker of a workspace on disk. It returns no
// engine for a workspace in memory, because the checker runs the go command
// in the root directory.
func checking(w lang.Workspace) ([]engine.Engine, error) {
	if !w.OnDisk() {
		return nil, nil
	}
	checked, err := checker.New(w.Root, Declaration())
	if err != nil {
		return nil, fmt.Errorf("go: %w", err)
	}
	return []engine.Engine{checked}, nil
}
