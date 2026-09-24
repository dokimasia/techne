// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java

import (
	_ "embed"
	"time"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-java/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a Java
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "java"

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

// Declaration returns the declaration of Java.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".java"},
		// The build files of Maven and Gradle, the two builds that jdtls
		// imports.
		Manifests: []string{
			"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
		},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
				{Open: "///"},
			},
		},
		IsTest:     IsTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of Java with the upstream tags
// query and the patterns of the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// server is the program of jdtls, which is also the name of the server.
const server = "jdtls"

// importing is how long a question waits for jdtls to settle. jdtls
// imports the Maven or Gradle build of the workspace before it answers.
const importing = 2 * time.Minute

// Server returns the declaration of jdtls, the Eclipse JDT language server.
// jdtls offers one extraction of the kind refactor.extract.function, which
// extracts a method.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		Loading:    importing,
		LanguageID: lsp.IdentityJava,
		Serves:     lsp.Binding(),
		Extracts:   lsp.Refactor{Kind: "refactor.extract.function"},
	}
}

// Register adds Java to r and its engines to c: the tree-sitter engine,
// and jdtls for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
