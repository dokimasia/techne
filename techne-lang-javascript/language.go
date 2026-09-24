// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript

import (
	_ "embed"
	"io/fs"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a
// JavaScript declaration contains it, so a change invalidates the IDs an
// index has stored.
const Language source.Language = "javascript"

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

// Declaration returns the declaration of JavaScript.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".js", ".mjs", ".cjs", ".jsx"},
		// The manifest of an npm package, and the file that marks the root of
		// a JavaScript project for the TypeScript server.
		Manifests: []string{"package.json", "jsconfig.json"},
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

// Grammar returns the tree-sitter grammar of JavaScript with the upstream
// tags query and the patterns of the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// The program of typescript-language-server, which is also the name of the
// server, and the flag that selects LSP over stdio.
const (
	server = "typescript-language-server"
	stdio  = "--stdio"
)

// Server returns the declaration of typescript-language-server, which
// serves JavaScript with the TypeScript compiler.
//
// A .jsx file opens as javascriptreact, because the server reads JSX in a
// file opened as javascript as an error. The server offers to extract a
// function in module scope and a method in the class, both of the kind
// refactor.extract.function. The declaration prefers the method, because
// a function in module scope cannot use the receiver.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, stdio},
		LanguageID: lsp.IdentityJavaScript,
		Dialects:   map[string]string{".jsx": lsp.IdentityJavaScriptReact},
		Serves:     lsp.Binding(),
		Extracts: lsp.Refactor{
			Kind:   "refactor.extract.function",
			Titles: []string{"method in class", "function in module scope"},
		},
		// typescript-language-server places a JavaScript file in a project
		// without a configuration file unless a tsconfig.json with allowJs, or a
		// jsconfig.json, includes it. Most JavaScript repositories have neither.
		// Such a project contains the open files and what they import.
		Scoped: true,
	}
}

// native is the program of TypeScript 7, which serves LSP itself.
const native = "tsc"

// Native returns the declaration of the language server of TypeScript 7,
// tsc --lsp, under the JavaScript identifier. TypeScript 7 ships no tsserver,
// which typescript-language-server runs, so a workspace on TypeScript 7
// takes this server. It offers no code action that extracts a function.
func Native() lsp.Server {
	return lsp.Server{
		Name:       native,
		Command:    []string{native, "--lsp", stdio},
		LanguageID: lsp.IdentityJavaScript,
		Dialects:   map[string]string{".jsx": lsp.IdentityJavaScriptReact},
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

// Register adds JavaScript to r and its engines to c: the tree-sitter
// engine, and for a workspace on disk the server that [For] returns.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), For(w.FS))
}
