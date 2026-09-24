// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lang defines what a language is, independently of the engines
// that serve it, and the file rules every engine applies.
//
// # Declarations
//
// A language module constructs one [Declaration] and registers it with
// [Registry.Register] from a composition root. The declaration states the
// facts that hold for every engine of the language: its extensions, its
// project manifests, its comment forms, and its rules for test files, units
// and visibility. Register checks the declaration and its engines before it
// changes anything.
//
// # Conventions
//
// [Stem], [JavaScriptTest] and [VisibilityByModifier] implement rules that
// more than one language shares. A language that declares visibility with a
// modifier, such as public in Java or pub in Rust, uses
// VisibilityByModifier, because a name alone does not show the modifier.
//
// # Files
//
// [Walk] returns the files of a scope that a language claims, with the
// files larger than [Largest] in a separate list. It skips the directories
// [Vendored] names and the paths the .gitignore files of the workspace
// exclude, by the rules of gitignore(5). [Readable] applies the .gitignore
// rules and the size limit to one path.
//
// # Lowered answers
//
// [Lowered] returns the tier of an answer that a type checker computed in a
// project with errors. A type checker binds every name that it can in such a
// project. Each use that it cannot bind is on a line that it reports as an
// error. An error lowers the answer to
// [go.dokimi.dev/techne/core/trust.Indexed] only when a [Hides] rule reports
// its line:
//
//   - [Writing] reports a line outside the sites of the answer that writes
//     the name the answer is about.
//   - [Binding] reports the line of a definition, and every line when the
//     definition found nothing.
//   - [Everywhere] reports every line.
//   - [Nowhere] does not report a line. A plan that rewrites no reference
//     uses it.
//
// [Worded] finds a name as a word in a line, and [WordAt] returns the word
// at an offset.
//
// # Routing
//
// [Registry.LanguageOf] returns the language that claims a path by its
// extension. It reports false for a path whose extension no language
// claims.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/sema, core/source and
// core/trust. Language modules import lang, and lang does not import a
// language module.
package lang
