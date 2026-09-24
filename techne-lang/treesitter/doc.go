// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package treesitter implements the syntactic engine: one engine type that
// serves every language with a tree-sitter grammar.
//
// # Grammars and queries
//
// A language module supplies a [Grammar]: the compiled grammar and a tags
// query in the capture convention of the grammars' own tags queries. The
// engine contains no language-specific code. [KindOf] maps each definition
// capture to a [go.dokimi.dev/techne/core/sema.Kind], and [New] refuses a
// query with a definition capture without a kind.
//
// A pattern cannot exclude the shapes of a more specific pattern, so two
// patterns can match one declaration. The declaration takes the kind that
// [MoreSpecific] ranks higher.
//
// # Metadata
//
// The engine reads the metadata of a declaration from the tree, not from
// the query:
//
//   - the modifier keywords, which are the only place most languages write
//     visibility
//   - the annotations, as [go.dokimi.dev/techne/core/sema.Annotation]
//     values
//   - the documentation, in the forms the language's
//     [go.dokimi.dev/techne/lang.CommentStyle] declares
//   - the parent, the innermost declaration whose span contains it, which
//     [Parents] computes from spans alone
//   - the visibility, which the declaration of the language reads from the
//     name, except that a declaration no code outside its scope can name,
//     such as a parameter or a local variable, is
//     [go.dokimi.dev/techne/core/sema.Unexported]
//
// # Identities
//
// The ID of a declaration contains its qualified name, which
// [go.dokimi.dev/techne/core/sema.Qualify] builds:
//
//   - A declaration with a [Receiver] capture is qualified by the type that
//     the capture names, so the Go method `func (s *Store) Get()` is
//     Store.Get.
//   - Any other declaration is qualified by the qualified name of its
//     parent, so the method Get of the class Store is Store.Get.
//   - An import is not qualified, because its name is a path.
//
// Members of one name and kind in different containers of one unit have
// different IDs. Overloads in one container share one ID.
//
// # Evidence
//
// Every answer is [go.dokimi.dev/techne/core/trust.Syntactic]. A name that
// crosses a file is matched as text, so an empty answer never proves
// absence.
//
// # Dependency position
//
// Imports the standard library, core, lang and the tree-sitter binding. It
// is the only package of the module that needs cgo. It contains no grammar,
// so the conformance suite of each language module tests its behaviour.
package treesitter
