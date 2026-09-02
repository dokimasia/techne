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

// Language is the wire form this module claims. It reaches a caller and,
// through a sema identity, an index that outlives the process, so
// changing it invalidates stored data.
const Language source.Language = "java"

// Upstream's query, unchanged, followed by this module's own patterns.
// Keeping them in separate files makes upgrading a grammar a re-vendor
// and a diff review rather than a hand merge.

//go:embed queries/upstream.scm
var upstreamQuery string

//go:embed queries/extends.scm
var extendsQuery string

// Declaration states the facts about java that hold whichever
// engine serves it.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{".java"},
		Manifests:  []string{"pom.xml", "build.gradle"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{
				{Open: "/**", Close: "*/", Continuation: " * "},
				{Open: "///"},
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
		Language: ts.NewLanguage(binding.Language()),
		Tags:     upstreamQuery + "\n" + extendsQuery,
	}
}

// server is what runs this language's server.
//
// Named once and used twice, as the server's own name and as the command
// to run: a declaration that spelt them differently would report one
// thing about itself and start another. It needs no argument, because it
// speaks the protocol over stdio and does nothing else.
const server = "jdtls"

// importing is how long a question waits for this server to read the
// workspace. Far longer than the default, because what it reads is a
// build description rather than a set of files.
const importing = 2 * time.Minute

// Server is the language server this module declares.
//
// jdtls is the Eclipse JDT language server, which is what every Java
// editor outside IntelliJ runs. It writes its index into a data
// directory beside the workspace on first use, so the first question
// about a project costs more than the numbers here suggest.
//
// Declared whether or not it is installed. Told nothing, a caller
// concludes this language cannot be served at all; told the server is
// missing, it knows what to install.
func Server() lsp.Server {
	return lsp.Server{
		Name:       server,
		Command:    []string{server},
		Loading:    importing,
		LanguageID: lsp.IdentityJava,
		Serves:     lsp.Binding(),
		// jdtls offers one extraction under the kind, and calls it
		// extracting to a method: Java has no function to extract to.
		Extracts: lsp.Refactor{Kind: "refactor.extract.function"},
	}
}

// Register adds java to a registry and its engines to a catalogue.
//
// A composition root calls this. Which engines follow from a workspace
// is settled in one place rather than ten, so a language cannot end up
// served differently from its siblings by accident.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return engines.Register(w, r, c, Declaration(), Grammar(), Server())
}
