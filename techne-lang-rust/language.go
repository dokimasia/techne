// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a Rust
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "rust"

// extendsQuery is the tags query that the module compiles in place of the
// upstream query in queries/upstream.scm. Upstream captures structs,
// enums, unions and type aliases as @definition.class, and sema.Kind tells
// the four apart. The module keeps the upstream query to compare it with
// the next release of the grammar.
//
//go:embed queries/extends.scm
var extendsQuery string

// Declaration returns the declaration of Rust, whose comment forms include
// the inner forms //! and /*! that document a module from inside it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".rs"},
		// The manifest of a Cargo package, and the file that describes a
		// project of another build system to rust-analyzer.
		Manifests: []string{"Cargo.toml", "rust-project.json"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "///"},
				{Open: "/**", Close: "*/", Continuation: " * "},
				{Open: "//!", Inside: true},
				{Open: "/*!", Close: "*/", Continuation: " * ", Inside: true},
			},
		},
		Blank:      map[string]bool{"_": true},
		IsTest:     IsTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of Rust with the tags query of
// the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     extendsQuery,
	}
}

// server is the program of rust-analyzer, which is also the name of the
// server.
const server = "rust-analyzer"

// Server returns the declaration of rust-analyzer. rust-analyzer offers to
// extract a variable, a constant, a static and a function, all four of the
// kind refactor.extract, so the declaration selects the function by its
// title.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityRust,
		Serves:     lsp.Binding(),
		Extracts: lsp.Refactor{
			Kind: "refactor.extract", Titles: []string{"into function"},
		},
		// rust-analyzer does not report an undeclared lifetime, E0261, or a
		// returned reference that does not live long enough. rustc reports both.
		Unchecked: "lifetimes or borrows",
	}
}

// Register adds Rust to r and its engines to c: the tree-sitter engine,
// and rust-analyzer for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
