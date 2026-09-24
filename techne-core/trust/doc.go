// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package trust describes the evidence behind an answer and what a caller
// may conclude from it.
//
// # Fidelity and completeness
//
// [Fidelity] is how an answer was bound. [Completeness] is how much of the
// scope the engine examined. They vary independently: a language server
// binds names through a type checker while its index may still be loading.
// An empty answer proves absence only when both are at their strongest,
// which [SupportsNegativeClaim] checks.
//
// # Provenance
//
// Engines declare a fixed fidelity per role and return caveats with each
// answer. Services combine the two into a [Provenance]. Engines never build
// one, so they cannot overstate their evidence.
//
// # Status
//
// [Status] separates an answer that nothing could serve from one that ran
// and found nothing. [Status.Answered] reports whether an engine produced a
// payload.
//
// # Dependency position
//
// Imports the standard library, core/source, and core/internal/wire.
package trust
