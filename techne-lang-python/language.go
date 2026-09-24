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

// Language is the language this module declares. Every sema.ID of a Python
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "python"

// extendsQuery is the tags query that the module compiles in place of the
// upstream query in queries/upstream.scm. Upstream captures an assignment
// at module level as @definition.constant, and Python has no constant. The
// module keeps the upstream query to compare it with the next release of
// the grammar.
//
//go:embed queries/extends.scm
var extendsQuery string

// Declaration returns the declaration of Python, with the visibility rule
// of PEP 8 and the test files of pytest.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".py", ".pyi"},
		// The project files of PEP 621 and setuptools, and the configuration
		// file of pyright.
		Manifests: []string{"pyproject.toml", "setup.py", "setup.cfg", "pyrightconfig.json"},
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
		Namespace:  lang.Stem,
		Visibility: Visibility,
	}
}

// Grammar returns the tree-sitter grammar of Python with the tags query of
// the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     extendsQuery,
	}
}

// The program of pyright's language server, which is also the name of the
// server, and the flag that selects LSP over stdio.
const (
	server = "pyright-langserver"
	stdio  = "--stdio"
)

// Server returns the declaration of pyright, the type checker of the
// Python extension of Visual Studio Code.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server, stdio},
		LanguageID: lsp.IdentityPython,
		Serves:     lsp.Binding(),
	}
}

// Register adds Python to r and its engines to c: the tree-sitter engine,
// and pyright for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
