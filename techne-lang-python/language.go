// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python

import (
	_ "embed"
	"io/fs"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-python/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the wire form this module claims. It reaches a caller and,
// through a sema identity, an index that outlives the process, so
// changing it invalidates stored data.
const Language source.Language = "python"

//go:embed queries/tags.scm
var tagsQuery string

// Declaration states the facts about python that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".py", ".pyi"},
		Manifests:  []string{"pyproject.toml", "setup.py"},
		Comment:    lang.CommentStyle{Line: "# ", Above: false},
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
		Tags:     tagsQuery,
	}
}

// Register adds python to a registry and its engines to a catalogue.
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
