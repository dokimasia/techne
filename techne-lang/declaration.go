// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Declaration states the facts about a language that hold whichever
// engine serves it. A language module constructs exactly one.
//
// These are facts about the language, not about a grammar. A language
// served only by a language server declares where its test files are
// without constructing a parser it never uses.
//
// Every field is required. [Registry.Register] refuses an incomplete
// declaration, so a language module's mistake surfaces at startup rather
// than as a nil call on the first request.
type Declaration struct {
	// Language is the wire form this module claims. Two modules claiming
	// one value are refused.
	Language source.Language

	// Extensions are the file suffixes that select this language, each
	// including its leading dot. Without one, nothing routes to the
	// module.
	Extensions []string

	// Manifests are the filenames marking a project root, such as
	// "go.mod" or "pyproject.toml".
	Manifests []string

	// Comment describes how a documentation comment attaches to a
	// declaration. No grammar states it and the document operations need
	// it.
	Comment CommentStyle

	// IsTest reports whether a path holds tests rather than shipped code.
	IsTest func(path string) bool

	// Namespace maps a file path to the name an import would use for it:
	// a/b/c.py becomes a.b.c.
	Namespace func(path string) string

	// Visibility reports whether a declared name can be seen outside the
	// unit that declares it, and returns [sema.VisibilityUnknown] where
	// the name does not say.
	//
	// Go and Python spell visibility in the name, so this answers for
	// them. Java, Rust and TypeScript spell it as a modifier or a
	// keyword that a name carries nothing of, so those return unknown
	// rather than reporting every declaration as public.
	Visibility func(name string) sema.Visibility
}

// CommentStyle describes how a documentation comment attaches to a
// declaration.
type CommentStyle struct {
	// Line prefixes each line of a comment block, including any trailing
	// space: "// " for Go, "# " for Python.
	Line string

	// Above reports whether the comment sits above the declaration
	// rather than inside it.
	Above bool
}
