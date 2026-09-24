// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package mock declares a line-oriented language whose [Engine] reads its files itself and
// serves the roles outline, search, resolve, relate, plan, check and verify at the tier that
// its registration sets. Tests and a composition root register it to exercise the tools of
// techne at tiers that no language module claims for every role.
//
// # Registration
//
// [Registering] returns the registration of a mock language of a name, which claims the
// extension of that name, so a composition root can register two or more that route apart.
// These options set what a language claims:
//
//   - [At] sets the tier of every role, [go.dokimi.dev/techne/core/trust.Resolved] by default.
//   - [Covering] sets the completeness of every answer,
//     [go.dokimi.dev/techne/core/trust.ScopeTotal] by default.
//   - [Missing] makes [Engine.Available] return an error with a reason.
//   - [Costing] sets the cost of every role,
//     [go.dokimi.dev/techne/core/engine.CostAnalyze] by default.
//
// [Register] registers one mock language, named [Language].
//
// # The language
//
// [Parse] reads a file as a list of lines. A line is blank, documentation that starts with ;;,
// a declaration of a word of [Kinds] and a name, or a use of a name. Two spaces indent a line
// one level into the declaration above it.
//
//	;; Store maps a name to an item.
//	type Store
//	  field size
//	func New
//	  use Store
//
// # Answers
//
// The roles do not keep state between calls. Each call reads what it needs:
//
//   - Outline and Search read the files of the language in the scope of the request.
//   - Resolve, Relate and Plan read every file of the language in the workspace, because a use
//     can refer to a declaration of any file.
//   - Verify checks the uses in the scope against the declarations of every file.
//   - Check parses the content of a change and does not read a file.
//
// An answer for a scope without a file of the language is skipped. A file larger than
// [go.dokimi.dev/techne/lang.Largest] is not read, and an answer that needs it is partial. An
// answer at the resolved tier has a [go.dokimi.dev/techne/core/trust.CaveatDynamic] caveat for
// the names that a program builds at run time. Relate declines an ID that no declaration has,
// and Plan refuses a target that does not identify a declaration.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/engine, core/sema, core/source,
// core/trust and lang. It does not need cgo or a grammar.
package mock
