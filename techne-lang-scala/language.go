// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala

import (
	_ "embed"
	"time"

	ts "github.com/tree-sitter/go-tree-sitter"
	binding "github.com/tree-sitter/tree-sitter-scala/bindings/go"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Language is the language this module declares. Every sema.ID of a Scala
// declaration contains it, so a change invalidates the IDs an index has
// stored.
const Language source.Language = "scala"

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

// Declaration returns the declaration of Scala.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".scala", ".sc"},
		// The build files of sbt, Mill, Scala CLI, Maven and Gradle, the
		// builds that Metals imports.
		Manifests: []string{
			"build.sbt", "build.mill", "build.mill.yaml", "build.sc", "project.scala",
			"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
		},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
			},
		},
		Blank:      map[string]bool{"_": true},
		IsTest:     IsTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// Grammar returns the tree-sitter grammar of Scala with the upstream tags
// query and the patterns of the module.
func Grammar() treesitter.Grammar {
	return treesitter.Grammar{
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// server is the program of Metals, which is also the name of the server.
const server = "metals"

// importing is how long a question waits for Metals to settle. Metals
// imports the build of the workspace before it answers.
const importing = 2 * time.Minute

// Server returns the declaration of Metals, the language server of Scala.
//
// Metals asks the client whether to import the build, and a client
// without a person to answer leaves Metals without a build. The settings
// tell Metals to import every build without asking. Metals reads them as
// the initialization options and in reply to workspace/configuration.
func Server() lsp.Server {
	return lsp.Server{
		Name:    server,
		Command: []string{server},
		Settings: map[string]any{
			"metals": map[string]any{"autoImportBuilds": "all"},
		},
		Loading:    importing,
		LanguageID: lsp.IdentityScala,
		Serves:     lsp.Binding(),
	}
}

// Register adds Scala to r and its engines to c: the tree-sitter engine,
// and Metals for a workspace on disk.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
