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

// Language is the wire form this module claims. It reaches a caller and,
// through a sema identity, an index that outlives the process, so
// changing it invalidates stored data.
const Language source.Language = "rust"

// Upstream's query is vendored for diffing but not compiled in. It tags
// a struct, an enum, a union and a type alias all @definition.class,
// which collapses four shapes the vocabulary keeps apart. Concatenating
// it would leave the kind of every Rust ADT to whichever pattern matched
// first, because class and enum rank alike. Everything upstream captures
// is covered below.

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about rust that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".rs"},
		Manifests:  []string{"Cargo.toml"},
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
		Namespace:  Namespace,
		Visibility: Visibility,
	}
}

// Grammar pairs the compiled grammar with the tags query vendored from
// upstream.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     extendsQuery,
	}
}

// server is what runs this language's server.
//
// Named once and used twice, as the server's own name and as the command
// to run: a declaration that spelt them differently would report one
// thing about itself and start another. It needs no argument, because it
// speaks the protocol over stdio and does nothing else.
const server = "rust-analyzer"

// Server is the language server this module declares.
//
// rust-analyzer is the official server and speaks the protocol over
// stdio with no subcommand.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityRust,
		Serves:     lsp.Binding(),
		// rust-analyzer offers extracting a variable, a constant, a
		// static and a function, all four under refactor.extract, so
		// the wording is what tells them apart.
		Extracts: lsp.Refactor{
			Kind: "refactor.extract", Titles: []string{"into function"},
		},
	}
}

// Register adds rust to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
