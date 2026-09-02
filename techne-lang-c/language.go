// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-c/bindings/go"
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
const Language source.Language = "c"

// Upstream's query is vendored for diffing but not compiled in. It
// captures a function by its declarator, so the span it reports stops
// before the body and nothing written inside a function nests under it.
// Everything upstream captures is covered below.

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about c that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".c", ".h"},
		Manifests:  []string{"Makefile", "CMakeLists.txt"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
				{Open: "/*!", Close: "*/", Continuation: " * "},
				{Open: "///"},
				{Open: "//!"},
			},
		},
		IsTest:     IsTest,
		Namespace:  Namespace,
		Visibility: Visibility,
	}
}

// Grammar pairs the compiled grammar with this module's tags query.
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
const server = "clangd"

// Server is the language server this module declares.
//
// clangd is part of LLVM and reads compile_commands.json to learn how
// each file is built. Without one it falls back to guessing the flags,
// and answers about a translation unit that may not be the real one.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityC,
		Serves:     lsp.Binding(),
	}
}

// Register adds c to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
