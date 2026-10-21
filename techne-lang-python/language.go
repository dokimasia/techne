// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package python

import (
	_ "embed"
	"os"
	"path/filepath"

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

// The delimiters of a docstring: three double quotes, and three single quotes.
const (
	tripleDouble = `"""`
	tripleSingle = `'''`
)

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
				{Open: tripleDouble, Close: tripleDouble, Inside: true},
				{Open: tripleSingle, Close: tripleSingle, Inside: true},
				{Open: "r" + tripleDouble, Close: tripleDouble, Inside: true},
				{Open: "r" + tripleSingle, Close: tripleSingle, Inside: true},
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
		// pyright resolves imports with the interpreter of the setting
		// python.pythonPath, or with the python on PATH without the setting.
		Configure: interpreter,
		// pyright returns the file of the imported module at the end of the module of an import.
		Imports: true,
		// pyright publishes the diagnostics of the open files alone under its
		// default diagnosticMode.
		OpenFilesOnly: true,
		// pyright reloads a changed pyproject.toml or pyrightconfig.json in
		// the background.
		Restarts: true,
	}
}

// The names under which a workspace has a virtual environment.
const (
	// venvDirectory is the directory in the root of a project in which uv
	// creates the virtual environment of the project.
	venvDirectory = ".venv"

	// venvVariable is the environment variable that the activation of a
	// virtual environment sets to the directory of the environment.
	venvVariable = "VIRTUAL_ENV"
)

// interpreters are the paths of the interpreter of a virtual environment in
// the layouts of POSIX and of Windows. Each path is relative to the
// directory of the environment.
var interpreters = []string{filepath.Join("bin", "python"), filepath.Join("Scripts", "python.exe")}

// The names under which pyright reads the interpreter from its settings.
const (
	// pythonSection is the section of the settings that contains the
	// interpreter.
	pythonSection = "python"

	// interpreterKey is the key of the interpreter in pythonSection.
	interpreterKey = "pythonPath"
)

// interpreter returns the setting python.pythonPath of pyright for the
// workspace at root. The value of the setting is the path of the first
// regular file of [interpreters] under the .venv directory of root, or else
// under the directory in VIRTUAL_ENV. getenv returns the value of
// VIRTUAL_ENV. interpreter returns nil when neither directory has an
// interpreter. The path keeps a symbolic link as it is, because Python finds
// its virtual environment from the path that starts it.
func interpreter(root string, getenv func(string) string) map[string]any {
	for _, venv := range []string{filepath.Join(root, venvDirectory), getenv(venvVariable)} {
		if venv == "" {
			continue
		}
		for _, program := range interpreters {
			full := filepath.Join(venv, program)
			if info, err := os.Stat(full); err == nil && info.Mode().IsRegular() {
				return map[string]any{pythonSection: map[string]any{interpreterKey: full}}
			}
		}
	}
	return nil
}

// Register adds Python to r and its engines to c: the tree-sitter engine,
// and pyright for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
