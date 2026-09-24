// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript

import (
	_ "embed"
	"io/fs"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a
// TypeScript declaration contains it, so a change invalidates the IDs an
// index has stored.
const Language source.Language = "typescript"

// extendsQuery is the tags query of the module, for the forms of JavaScript
// that the TypeScript grammar inherits and the forms that TypeScript adds.
// The module keeps the two upstream queries in queries to compare them with
// the next release of the grammar, and does not compile them, because the
// only constant pattern of the JavaScript query captures an assignment in
// an export statement and no pattern captures a plain const.
//
//go:embed queries/extends.scm
var extendsQuery string

// Declaration returns the declaration of TypeScript. It claims .tsx,
// because the server and the type checker read a .tsx file as TypeScript.
// A rename that crosses the two extensions stays within one language.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".ts", ".mts", ".cts", ".tsx"},
		// The manifest of an npm package, and the configuration file of the
		// TypeScript compiler.
		Manifests: []string{"package.json", "tsconfig.json"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
			},
		},
		IsTest:     lang.JavaScriptTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of TypeScript with the tags
// query of the module. A .tsx file takes the TSX grammar of the same
// binding, because the TypeScript grammar parses JSX as an error.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.LanguageTypescript()),
		Dialects: map[string]*ts.Language{
			".tsx": ts.NewLanguage(binding.LanguageTSX()),
		},
		Tags: extendsQuery,
	}
}

// The program of typescript-language-server, which is also the name of the
// server, and the flag that selects LSP over stdio.
const (
	server = "typescript-language-server"
	stdio  = "--stdio"
)

// Server returns the declaration of typescript-language-server. The
// JavaScript module declares the same program under the JavaScript
// identifier, so each language has its own engine and its own server.
//
// A .tsx file opens as typescriptreact, because the server reads JSX in a
// file opened as typescript as an error. The server offers to extract a
// function in module scope and a method in the class, both of the kind
// refactor.extract.function. The declaration prefers the method, because
// a function in module scope cannot use the receiver.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, stdio},
		LanguageID: lsp.IdentityTypeScript,
		Dialects:   map[string]string{".tsx": lsp.IdentityTypeScriptReact},
		Serves:     lsp.Binding(),
		Extracts: lsp.Refactor{
			Kind:   "refactor.extract.function",
			Titles: []string{"method in class", "function in module scope"},
		},
		// typescript-language-server places a file that no tsconfig.json
		// includes in a project without a configuration file, which contains
		// the open files and what they import. The engine cannot see which
		// project contains a file, so it opens the files that write a name for
		// every rename and move.
		Scoped: true,
	}
}

// native is the program of TypeScript 7, which serves LSP itself.
const native = "tsc"

// Native returns the declaration of the language server of TypeScript 7,
// tsc --lsp. TypeScript 7 ships no tsserver, which typescript-language-server
// runs, so a workspace on TypeScript 7 takes this server. It offers no code
// action that extracts a function, and it places a file that no
// tsconfig.json includes in a project without a configuration file, as
// tsserver does.
func Native() lsp.Server {
	return lsp.Server{
		Name:       native,
		Command:    []string{native, "--lsp", stdio},
		LanguageID: lsp.IdentityTypeScript,
		Dialects:   map[string]string{".tsx": lsp.IdentityTypeScriptReact},
		Serves:     lsp.Binding(),
		Scoped:     true,
	}
}

// For returns the server of the workspace of fsys: [Native] when the
// workspace uses TypeScript 7 or later, and [Server] otherwise.
func For(fsys fs.FS) lsp.Server {
	if major, found := lang.NodeMajor(fsys, "typescript"); found && major >= 7 {
		return Native()
	}
	return Server()
}

// Register adds TypeScript to r and its engines to c: the tree-sitter
// engine, and for a workspace on disk the server that [For] returns.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), For(w.FS))
}
