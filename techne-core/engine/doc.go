// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package engine defines the ports an engine implements and the catalogue
// that selects engines for a request.
//
// # Roles and ports
//
// Each [Role] has its own port interface, such as [Outliner] or [Planner].
// An engine that cannot serve a role does not implement its port, and the
// catalogue selects engines by type assertion. An engine can also decline a
// role of a port it implements by declaring [trust.None] for it.
//
// # Evidence
//
// An engine declares a fixed fidelity and cost per role and returns a
// [Result] with its items, coverage, and caveats. Services turn the result
// into an [Answer] with [Publish], which takes the tier from the engine. An
// engine cannot overstate its evidence.
//
// # Selection
//
// [Catalog.For] orders the engines of a language and role by fidelity, then
// by cost. [Ask] tries them in order. [ErrDecline] moves on to the next
// engine. Any other error stops the search, because a weaker answer would
// hide a broken engine.
//
// [AskEach] and [AskAny] apply Ask to the languages of a request:
//
//   - AskEach asks every language for the read path. A failed language is
//     recorded in [Declined], and the other languages still answer.
//   - AskAny returns the first answer for the write path. Any error stops
//     it, because the next language can answer about a different
//     declaration.
//
// # Dependency position
//
// Imports the standard library, core/edit, core/sema, core/source,
// core/trust, and core/internal/wire.
package engine
