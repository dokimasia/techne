// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package edit describes how code changes: which operations exist, what
// each may be pointed at, and the evidence each needs to be correct.
//
// # Every operation is declared
//
// [Operations] lists the catalogue whether or not any language can serve
// it. A caller asking for one no language implements is told that
// language cannot do it, rather than that no such operation exists: the
// first is a capability gap to route around, the second reads as a typo.
//
// # Specs are checked before a planner is chosen
//
// [SpecFor] returns what an operation declares about itself. A request
// is validated against that before any language is consulted, so a
// malformed request produces one consistent refusal whatever would have
// served it.
//
// # Rewriting references demands more than binding
//
// [Spec.MinFidelity] is the weakest [trust.Fidelity] an operation can be
// correct on. [Spec.RewritesReferences] marks an operation that also
// changes the code referring to its target; finding every reference is a
// claim that no others exist, so admitting one demands total coverage as
// well. The spec rules out the tiers that can never be enough, and the
// coverage an engine reports decides the rest.
//
// # Naming
//
// An operation is named family.subject. [Operation.Family] returns the
// part before the dot, so a family can be advertised or refused as a
// unit and a caller can ask what extractions exist without enumerating
// subjects.
//
// # Dependency position
//
// Imports the standard library, core/sema, core/source and core/trust.
// Planners in a language module produce values described here; the write
// path consumes them.
package edit
