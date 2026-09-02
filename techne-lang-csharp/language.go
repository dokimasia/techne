// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
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
const Language source.Language = "csharp"

// Upstream's query, unchanged, followed by this module's own patterns.
// Keeping them in separate files makes upgrading a grammar a re-vendor
// and a diff review rather than a hand merge.

//go:embed queries/upstream.scm
var upstreamQuery string

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about csharp that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".cs"},
		Manifests:  []string{"*.csproj", "*.sln"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "///"},
				{Open: "/**", Close: "*/", Continuation: " * "},
			},
		},
		Blank:      map[string]bool{"_": true},
		IsTest:     IsTest,
		Namespace:  Namespace,
		Visibility: Visibility,
	}
}

// Grammar pairs the compiled grammar with this module's tags query.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// server is what runs this language's server.
//
// Named once and used twice, as the server's own name and as the command
// to run: a declaration that spelt them differently would report one
// thing about itself and start another. It needs no argument, because it
// speaks the protocol over stdio and does nothing else.
const server = "csharp-ls"

// Server is the language server this module declares.
//
// csharp-ls wraps Roslyn and speaks the protocol over stdio. The
// server Microsoft ships with its own editor extension talks over a
// named pipe and is documented as not standing alone, so it cannot be
// driven the way every other server here is.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityCSharp,
		Serves:     lsp.Binding(),
	}
}

// Register adds csharp to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
