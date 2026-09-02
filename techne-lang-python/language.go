// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-python/bindings/go"
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
const Language source.Language = "python"

// Upstream's query is vendored for diffing but not compiled in, which
// is the exception rather than the rule here. It tags every module-level
// assignment @definition.constant, and Python has no constant: what it
// calls one is a variable that nothing reassigns by convention. Keeping
// it would report `registry = {}` as a constant, which CPython's own
// symbol table disagrees with.

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about python that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".py", ".pyi"},
		Manifests:  []string{"pyproject.toml", "setup.py"},
		Comment: lang.CommentStyle{
			Line: "# ",
			Doc: []lang.DocStyle{
				{Open: `"""`, Close: `"""`, Inside: true},
				{Open: `'''`, Close: `'''`, Inside: true},
				{Open: `r"""`, Close: `"""`, Inside: true},
				{Open: `r'''`, Close: `'''`, Inside: true},
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
		Language: ts.NewLanguage(binding.Language()),
		Tags:     extendsQuery,
	}
}

// What runs this language's server.
//
// The program is named once and used twice, as the server's own name and
// as the command to run: a declaration that spelt them differently would
// report one thing about itself and start another.
const (
	server = "pyright-langserver"
	// stdio is the flag that speaks the protocol over stdin and stdout rather than
	// over a socket or a node channel.
	stdio = "--stdio"
)

// Server is the language server this module declares.
//
// pyright is the type checker behind the Python extension for Visual
// Studio Code, and the only widely installed one that binds names
// rather than guessing at them.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, stdio},
		LanguageID: lsp.IdentityPython,
		Serves:     lsp.Binding(),
	}
}

// Register adds python to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
