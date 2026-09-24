// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package edit describes code changes: the operations a caller can request,
// the evidence each requires, and the plans that planners produce.
//
// # Operations
//
// [Operations] lists every operation, including those no language
// implements, so a caller can tell a language that cannot do something from
// an operation that does not exist. Operations are named family.subject, and
// [Operation.Family] returns the family.
//
// # Specs
//
// [SpecFor] returns what an operation declares: the targets it accepts, its
// arguments, and [Spec.MinFidelity], the weakest evidence it is correct on.
// An operation with [Spec.RewritesReferences] also requires total coverage,
// because finding every reference is a claim that there are no others.
// Services validate each request against its spec before asking a language.
//
// # Plans and policy
//
// A [Plan] has no file handle, so it can be inspected and discarded without
// touching the workspace. [Policy.Admit] decides whether a plan may be
// applied. [Apply] computes the bytes a change produces. Planners and the
// write path both call it, so a preview and the applied change agree.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/sema, core/source, and
// core/trust.
package edit
