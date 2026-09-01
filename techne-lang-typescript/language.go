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

// Register adds typescript to a registry and its engines to a catalogue.
//
// A composition root calls this. Reading the workspace through an
// [io/fs.FS] keeps every path relative and lets a caller serve a tree
// that is not on disk.
func Register(fsys fs.FS, r *lang.Registry, c *engine.Catalog) error {
	parser, err := treesitter.New(fsys, Declaration(), Grammar())
	if err != nil {
		return err
	}
	return r.Register(c, Declaration(), parser)
}
