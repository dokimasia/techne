// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package query serves the read roles of techne: outline, search, resolve, relate and
// verify. [Service] asks the engines of each language that a request is about and merges
// their answers.
//
// # One path for every role
//
// The read roles take the same steps. [engine.AskEach] selects the languages of the scope
// and asks the engines of each language strongest first, and the service merges the
// answers. A merged answer claims the weakest tier and the weakest completeness of the
// answers that read a file of the scope, and names each engine behind them.
//
// # Declines, skips and failures
//
//   - An engine that returns [engine.ErrDecline] serves the role but not this request, so
//     the next engine of its language is asked. A language whose engines all decline makes
//     the merged answer partial, with a caveat that contains their reasons.
//   - A skipped answer states that the scope contains no file of its language, and the
//     merge leaves it out of the evidence.
//   - Any other error stops the engines of its language, because a weaker answer would hide
//     a broken engine. The other languages still answer.
//
// # Unsupported answers
//
// These requests return [trust.Unsupported] with a caveat that contains the reason, and no
// error:
//
//   - a language without an engine
//   - a path of an extension that the router does not claim
//   - a scope without an answer, or where every answer is skipped and a language declined
//
// # Dependency position
//
// Imports the standard library, core/edit, core/engine, core/sema, core/source and
// core/trust.
package query
