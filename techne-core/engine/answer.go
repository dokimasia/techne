// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Answer is a published result: the items an engine returned and the
// evidence behind them. Services build an Answer with [Publish]. Engines
// return a [Result] and never build an Answer.
//
// Empty Items do not prove absence. [trust.Status.Answered] reports whether
// an engine ran, and [trust.Provenance.SupportsNegativeClaim] reports whether
// an empty list proves that none exist.
type Answer[T any] struct {
	Items      []T
	Status     trust.Status
	Provenance trust.Provenance

	// Skipped is [Result.Skipped] of the engine that answered. Services use it
	// to leave the answer out of a merge across languages. Callers read
	// Provenance.
	Skipped bool
}

// Request is the scope of one question, and the number of relations that the
// caller keeps from the answer.
type Request struct {
	// Scope is a file or a directory, relative to the workspace root.
	Scope source.Path

	// Language restricts the request to one language. Empty selects the
	// languages that [Languages] returns for Scope.
	Language source.Language

	// Preferred is the lowest tier the caller wants. An answer below it is
	// published as [trust.Degraded], not refused.
	Preferred trust.Fidelity

	// Tests includes the files the language treats as tests. The language
	// module defines which paths those are.
	Tests bool

	// Limit is the number of relations that the caller keeps from
	// [Relator.Relate], and zero keeps every relation. An engine may return
	// only the first Limit relations by the path and offset of their sites. It
	// then adds a [trust.CaveatTruncated] caveat that counts the relations it
	// found.
	Limit int
}

// Query is a search for declarations.
type Query struct {
	// Text matches declaration names by fuzzy match and documentation by
	// content.
	Text string

	// Kind restricts matches to one kind. KindUnknown matches every kind.
	Kind sema.Kind

	// Private includes declarations that are not visible outside their
	// unit.
	Private bool

	// Limit caps the number of items. Zero selects the engine's default.
	Limit int
}
