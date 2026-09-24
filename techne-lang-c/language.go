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

// Language is the language this module declares. Every sema.ID of a C
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "c"

// extendsQuery is the tags query that the module compiles in place of the
// upstream query in queries/upstream.scm. Upstream captures a function by
// its declarator, so the span of a function ends before its body and no
// declaration of the body nests under the function. The module keeps the
// upstream query to compare it with the next release of the grammar.
//
//go:embed queries/extends.scm
var extendsQuery string

// Declaration returns the declaration of C. A header is C, so C claims .h.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".c", ".h"},
		// The build files of CMake, make and Meson, and the files from which
		// clangd reads the compile flags of a project.
		Manifests: []string{
			"CMakeLists.txt", "GNUmakefile", "Makefile", "makefile", "meson.build",
			"compile_commands.json", "compile_flags.txt",
		},
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
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of C with the tags query of the
// module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     extendsQuery,
	}
}

// server is the program of clangd, which is also the name of the server.
const server = "clangd"

// Server returns the declaration of clangd, the language server of LLVM.
// clangd reads the compile flags of each file from compile_commands.json,
// and guesses the flags of a file that the database does not list.
//
// The declaration omits check. clangd builds the preamble of a file from
// the headers on disk, so a check of an unwritten change to a header
// reports every file that includes the header as broken. The tree-sitter
// engine checks a change to C.
func Server() lsp.Server {
	serves := lsp.Binding()
	delete(serves, engine.RoleCheck)
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityC,
		Serves:     serves,
	}
}

// Register adds C to r and its engines to c: the tree-sitter engine, and
// clangd for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
