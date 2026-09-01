// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lang declares what a language is, independently of which
// engine answers questions about it.
//
// # Facts about a language, not about a grammar
//
// [Declaration] holds what stays true whichever engine serves the
// language: which suffixes select it, where its project roots are, how
// it writes comments and documentation, which identifiers bind nothing,
// which files hold tests, how a path becomes a namespace, and what makes
// a name visible outside its unit. A language served only by a language
// server states all of it without constructing a parser it never uses.
//
// # Documentation has more than one form
//
// [CommentStyle.Doc] is a list rather than a pair of fields, because a
// language has several documentation forms and they are not
// interchangeable. Rust has four, Java and C# two each, and C whatever
// Doxygen reads. One token does not mean one thing across languages
// either: /// is documentation in Java, Rust, C and C#, and a compiler
// directive in TypeScript. [CommentStyle.Documents] is the form to
// write; [CommentStyle.Documentation] recognises any of them.
//
// # Registration is a call, not an import
//
// [Registry.Register] is called once per language from a composition
// root. Nothing registers from an init function, so the set of languages
// is a value the caller chooses: a test builds a registry holding one
// language, and a smaller binary ships a subset of the same calls.
//
// # Refusal is total
//
// Register checks the declaration, the language, every extension and
// every engine before it touches the catalogue. A rejected module leaves
// no engines behind for a language nothing can route to.
//
// # Routing
//
// [Registry.LanguageOf] answers which language claims a path, by
// extension. It reports nothing for a suffix nobody claimed, because
// guessing would answer about a language nothing declared at a fidelity
// nothing earned.
//
// # Dependency position
//
// Imports core and nothing else in this repository. A language module
// imports this package; nothing here imports a language module, which is
// what lets a language be deleted as a unit.
package lang
