// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-go/bindings/go"
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
const Language source.Language = "go"

// Upstream's query, unchanged, followed by this module's own patterns.
// Keeping them in separate files makes upgrading a grammar a re-vendor
// and a diff review rather than a hand merge.

//go:embed queries/upstream.scm
var upstreamQuery string

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about go that hold whichever
// engine serves it.
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

// Grammar pairs the compiled grammar with the tags query vendored from
// upstream.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// What runs this language's server.
//
// The program is named once and used twice, as the server's own name and
// as the command to run: a declaration that spelt them differently would
// report one thing about itself and start another.
const (
	server = "gopls"
	// serves is the subcommand that speaks the protocol over stdio.
	serves = "serve"
)

// Server is the language server this module declares.
//
// gopls is the Go team's own server and the one every Go editor uses.
// The serve subcommand speaks the protocol over stdio.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, serves},
		LanguageID: lsp.IdentityGo,
		Serves:     lsp.Binding(),
	}
}

// Register adds go to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
