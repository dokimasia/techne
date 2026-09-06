// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
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
const Language source.Language = "javascript"

// Upstream's query, unchanged, followed by this module's own patterns.
// Keeping them in separate files makes upgrading a grammar a re-vendor
// and a diff review rather than a hand merge.

//go:embed queries/upstream.scm
var upstreamQuery string

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about javascript that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".js", ".mjs", ".cjs", ".jsx"},
		Manifests:  []string{"package.json"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
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
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// What runs this language's server.
//
// The program is named once and used twice, as the server's own name and
// as the command to run: a declaration that spelt them differently would
// report one thing about itself and start another.
const (
	server = "typescript-language-server"
	// stdio is the flag that speaks the protocol over stdin and stdout rather than
	// over a socket.
	stdio = "--stdio"
)

// Server is the language server this module declares.
//
// The TypeScript server checks JavaScript too, under the JavaScript
// identity: it is the same compiler with the type syntax turned off,
// and no separate JavaScript server is maintained.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, stdio},
		LanguageID: lsp.IdentityJavaScript,
		// The grammar parses JSX either way, and a server does not: a
		// .jsx file opened as javascript is one whose JSX a server reads
		// as an error.
		Dialects: map[string]string{".jsx": lsp.IdentityJavaScriptReact},
		Serves:   lsp.Binding(),
		// The same server as TypeScript, and the same menu: an inner
		// function that cannot see the receiver, offered first, beside
		// the method that can.
		Extracts: lsp.Refactor{
			Kind:   "refactor.extract.function",
			Titles: []string{"method in class", "function in module scope"},
		},
	}
}

// Register adds javascript to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
