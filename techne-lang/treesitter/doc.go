// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package treesitter answers about any language with a tree-sitter
// grammar, at the syntactic tier.
//
// # One engine, many grammars
//
// A language module supplies a grammar and the queries that go with it,
// and writes no engine code. What this package knows is the query
// convention, not any language.
//
// # The capture convention
//
// [Capture] names come from the tags queries the grammars already ship,
// so a module starts from upstream's query and extends it. [KindOf] maps
// a definition capture to what it declares and refuses anything else,
// and [New] refuses a query naming a definition capture no kind carries:
// a mistyped capture that reached the query would match and be dropped,
// producing an engine that silently finds nothing.
//
// Where a grammar draws a distinction the shared vocabulary does not
// carry, two captures map onto one kind. A caller reads the same set
// whichever language answered.
//
// A pattern cannot say what it is not, so the general pattern for a
// shape also matches the specific ones and both reach the engine for one
// declaration. [Outranks] decides which kind survives.
//
// # What a query does not capture
//
// Three things are read off the tree rather than out of a match, because
// each is attached to a declaration rather than named by it and a
// pattern per grammar per shape would be unreadable:
//
//   - the keywords qualifying it, which for most languages are the only
//     place visibility is written
//   - the metadata written onto it, which reaches a caller as
//     [go.dokimi.dev/techne/core/sema.Annotation] rather than as a
//     symbol, because applying an annotation binds no name
//   - its documentation, in whichever of the forms its
//     [go.dokimi.dev/techne/lang.CommentStyle] states, read from above
//     the declaration or from inside its body as the language decides
//
// [Parents] is the fourth, and is decided by span containment rather
// than by a query, so it holds for every grammar and for engines at any
// tier.
//
// # What this tier can and cannot say
//
// A parser matched text. Links that cross a file are name coincidence,
// so answers are [go.dokimi.dev/techne/core/trust.Syntactic] and an
// empty one never proves absence.
//
// # Dependency position
//
// Imports core, the tree-sitter binding and lang. It is the only package
// in this module that needs a C toolchain, so a binary registering only
// language-server-backed languages never reaches it. It holds no
// grammar, so the behaviour that needs one is checked by the conformance
// suite each language module runs.
package treesitter
