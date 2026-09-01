// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package engine is the contract an adapter implements to answer
// questions about code.
//
// # A role is declined by not having the method
//
// Each role is its own interface. An engine that cannot serve one omits
// the method rather than returning an error, and a service selects by
// type assertion. An engine claiming a role it cannot serve would
// advertise a capability that is not there, which a caller can only find
// out by calling it.
//
// # Engines do not build their own provenance
//
// [Engine.Fidelity] declares a fixed tier per role and an answer carries
// per-answer caveats. A service stamps the rest of the
// [trust.Provenance]. An adapter therefore cannot overstate its evidence,
// and the rule for what counts as degraded cannot drift between one role
// and another.
//
// # Fidelity is per role
//
// A language server commonly binds references through a type system
// while returning a document outline its own parser produced. Those are
// different claims, so [Engine.Fidelity] takes the [Role] being asked
// about.
//
// # Cost is independent of fidelity
//
// [Engine.Cost] says what producing an answer takes. A warm index and
// the parser that filled it produce the same facts at [CostMemory] and
// [CostParse]. [CostSession] describes an engine that is expensive once
// and cheap afterwards, so pricing it at its first call would make a
// caller avoid the fastest engine it has.
//
// # Declining one request is not failing
//
// An engine returns [ErrDecline] for a request it cannot serve, and a
// service moves to the next engine. Any other error stops selection,
// because answering from a weaker engine when the stronger one is broken
// hides the breakage for as long as anyone believes the answer.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/sema,
// core/source and core/trust. Language modules implement these
// interfaces; the read and write services consume them.
package engine
