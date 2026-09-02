// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
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
const Language source.Language = "typescript"

// The TypeScript grammar inherits the JavaScript one, so a .ts file uses
// both languages' forms. This module's own patterns cover both.
//
// Upstream's two files are vendored for diffing but not compiled in:
// JavaScript's only constant pattern is for the CommonJS export form, so
// a plain const was captured by nothing.

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about typescript that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".ts", ".mts", ".cts"},
		Manifests:  []string{"package.json", "tsconfig.json"},
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

// Grammar pairs the compiled grammar with the tags query vendored from
// upstream.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.LanguageTypescript()),
		Tags:     extendsQuery,
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
// One program serves TypeScript and JavaScript, so this declaration and
// the JavaScript module's name the same binary. They are two engines:
// each opens its files under its own identity, and a file opened as the
// wrong one is checked by the wrong rules.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, stdio},
		LanguageID: lsp.IdentityTypeScript,
		Serves:     lsp.Binding(),
	}
}

// Register adds typescript to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
