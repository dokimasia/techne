// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby

import (
	_ "embed"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a Ruby
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "ruby"

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

// Declaration returns the declaration of Ruby.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".rb", ".rake", ".gemspec"},
		// The Gemfile names that Bundler reads, the Rakefile names that rake
		// reads, and the specification of a gem.
		Manifests: []string{
			"Gemfile", "gems.rb", "Rakefile", "rakefile", "Rakefile.rb", "rakefile.rb", "*.gemspec",
		},
		Comment: lang.CommentStyle{
			Line: "# ", BlockOpen: "=begin", BlockClose: "=end",
			Doc: []lang.DocStyle{
				{Open: "#"},
				{Open: "=begin rdoc", Close: "=end"},
			},
		},
		IsTest:     IsTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of Ruby with the upstream tags
// query and the patterns of the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// server is the program of ruby-lsp, which is also the name of the server.
const server = "ruby-lsp"

// Server returns the declaration of ruby-lsp, the language server of
// Shopify. ruby-lsp runs under the Ruby and the bundle of the process that
// starts techne.
//
// ruby-lsp finds the references of a method by its name, so the uses of
// one method include the methods of other classes with that name. Its
// relations claim the indexed tier.
func Server() lsp.Server {
	serves := lsp.Binding()
	serves[engine.RoleRelate] = trust.Indexed
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		LanguageID: lsp.IdentityRuby,
		Serves:     serves,
	}
}

// Register adds Ruby to r and its engines to c: the tree-sitter engine,
// and ruby-lsp for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
