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

// Request is the scope of one question. Limit and Declared apply to
// [Relator.Relate] alone.
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

	// Declared is the span of the declaration that [Relator.Relate] starts
	// from, as the outline of the caller reports it, or the zero span. An
	// engine whose own declarations do not match the ID can find the
	// declaration at this span.
	Declared source.Span
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

	// Include selects the bindings that the search returns beside the
	// declarations that a file offers to the rest of a program. The engine
	// applies it before Limit.
	Include Bindings

	// Limit caps the number of items. Zero selects the engine's default.
	Limit int
}

// Bindings are the bindings that an answer contains beside the declarations
// that a file offers to the rest of a program. The zero value contains none
// of them.
type Bindings uint8

const (
	// BindImports contains what a file brings into scope.
	BindImports Bindings = 1 << iota
	// BindParameters contains the parameters and the type parameters of each
	// signature.
	BindParameters
	// BindLocals contains the declarations inside the body of a callable or
	// the value of a binding, which [sema.Locals] reports.
	BindLocals
	// BindLabels contains the labels of statements.
	BindLabels

	// BindAll contains every binding.
	BindAll = BindImports | BindParameters | BindLocals | BindLabels
)

// Keeps reports whether an answer with the bindings b contains a declaration
// of kind. local reports that the declaration is inside the body of a
// callable or the value of a binding. An import, a parameter, a type
// parameter and a label take their own binding before local applies.
func (b Bindings) Keeps(kind sema.Kind, local bool) bool {
	switch {
	case kind == sema.KindImport:
		return b&BindImports != 0
	case kind == sema.KindParameter, kind == sema.KindTypeParameter:
		return b&BindParameters != 0
	case kind == sema.KindLabel:
		return b&BindLabels != 0
	case local:
		return b&BindLocals != 0
	default:
		return true
	}
}
