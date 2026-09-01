// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala

import (
	_ "embed"
	"io/fs"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-scala/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the wire form this module claims. It reaches a caller and,
// through a sema identity, an index that outlives the process, so
// changing it invalidates stored data.
const Language source.Language = "scala"

// Upstream's query, unchanged, followed by this module's own patterns.
// Keeping them in separate files makes upgrading a grammar a re-vendor
// and a diff review rather than a hand merge.

//go:embed queries/upstream.scm
var upstreamQuery string

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about scala that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".scala", ".sc"},
		Manifests:  []string{"build.sbt", "build.sc"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
			},
		},
		Blank:      map[string]bool{"_": true},
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

// Register adds scala to a registry and its engines to a catalogue.
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
