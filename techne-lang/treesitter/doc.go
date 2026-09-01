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
// so a module vendors its query unchanged. [KindOf] maps a definition
// capture to what it declares and refuses anything else, because a
// mistyped capture that mapped to a kind would produce an engine that
// silently finds nothing.
//
// Where a grammar draws a distinction the shared vocabulary does not
// carry, two captures map onto one kind. A caller reads the same set
// whichever language answered.
//
// # What this tier can and cannot say
//
// A parser matched text. Links that cross a file are name coincidence,
// so answers are [trust.Syntactic] and an empty one never proves
// absence.
//
// # Dependency position
//
// Imports core, the tree-sitter binding and lang. It is the only package
// in this module that needs a C toolchain, so a binary registering only
// language-server-backed languages never reaches it.
package treesitter
