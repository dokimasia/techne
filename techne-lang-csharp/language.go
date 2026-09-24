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

// Language is the language this module declares. Every sema.ID of a C#
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "csharp"

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

// Declaration returns the declaration of C#.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".cs"},
		// A project file, and the solution files of the .NET SDK. The SDK
		// reads .slnx from version 9.0.200.
		Manifests: []string{"*.csproj", "*.sln", "*.slnx"},
		// Documentation is XML. The compiler and StyleCop read the
		// description of a declaration from its summary element.
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "///", Element: "summary"},
				{Open: "/**", Close: "*/", Continuation: " * ", Element: "summary"},
			},
		},
		Blank:      map[string]bool{"_": true},
		IsTest:     IsTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of C# with the upstream tags
// query and the patterns of the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// server is the program of csharp-ls, which is also the name of the server.
const server = "csharp-ls"

// Server returns the declaration of csharp-ls, a language server over
// Roslyn that speaks LSP over stdio.
//
// csharp-ls does not set a kind on a code action, so the declaration
// selects the extraction by its title. csharp-ls also offers to extract a
// local function, which only the method that contains it can call.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityCSharp,
		Serves:     lsp.Binding(),
		Extracts:   lsp.Refactor{Titles: []string{"extract method"}},
	}
}

// Register adds C# to r and its engines to c: the tree-sitter engine, and
// csharp-ls for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
