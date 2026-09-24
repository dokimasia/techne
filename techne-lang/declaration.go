// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Declaration states the facts about one language that hold for every
// engine that serves it. A language module constructs one and passes it to
// [Registry.Register].
//
// Register refuses a declaration without Language, without an extension,
// with an extension that has no leading dot, or without IsTest, Namespace or
// Visibility.
type Declaration struct {
	// Language is the value the module declares. Register refuses a value
	// that another module declared.
	Language source.Language

	// Extensions are the file extensions of the language, each with its
	// leading dot, such as ".go". Register refuses an extension that another
	// module declared.
	Extensions []string

	// Manifests are the names of the files that mark a project root, such as
	// "go.mod". A name can be a path.Match pattern, such as "*.csproj".
	Manifests []string

	// Comment is the comment and documentation syntax of the language.
	Comment CommentStyle

	// Blank lists the identifiers that do not bind a name, such as "_" in Go,
	// Rust and C#. Engines do not report a declaration of one.
	Blank map[string]bool

	// IsTest reports whether the file at path contains tests.
	IsTest func(path string) bool

	// Namespace returns the unit of the file at path: the path an import
	// names it by, such as a/b/c for a/b/c.py, or the directory a/b for
	// a/b/c.go in Go. [Stem] implements the rule for a language whose unit
	// is the file.
	Namespace func(path string) string

	// Visibility returns the visibility that a name encodes, or
	// sema.VisibilityUnknown for a language that declares visibility with a
	// modifier. [VisibilityByModifier] implements the second case.
	Visibility func(name string) sema.Visibility
}
