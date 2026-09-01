// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package trust describes the evidence behind an answer and what a
// caller may read into it.
//
// # Two axes, not one
//
// [Fidelity] says how an answer was bound. [Completeness] says how much
// of the requested scope the engine examined. They are independent: a
// language server binds names through a type system and may still hold a
// partial index. Reading an empty answer as proof of absence needs both,
// which is what [SupportsNegativeClaim] decides.
//
// # Who builds these values
//
// An engine declares a fixed fidelity and returns per-answer caveats. A
// service stamps the rest of [Provenance]. An adapter therefore cannot
// overstate its own evidence, and a service cannot discard a limit only
// the adapter knew about.
//
// # Empty is not one thing
//
// [Status] separates an answer nothing could serve from one that ran and
// matched nothing. [Status.Answered] reports whether a payload was
// produced at all, so an empty item list is never mistaken for evidence
// of absence.
//
// # Dependency position
//
// Imports the standard library and core/source. Every service, every
// engine and every language module names these types; nothing here
// imports a service.
package trust
