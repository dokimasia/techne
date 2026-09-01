// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Answer carries what an engine found and the evidence behind it.
//
// Items being empty means nothing on its own. Read [trust.Status.Answered]
// to learn whether an engine ran at all, and
// [trust.SupportsNegativeClaim] over the provenance to learn whether an
// empty result means there are none.
type Answer[T any] struct {
	Items      []T
	Status     trust.Status
	Provenance trust.Provenance
}

// Request is the scope of one question.
type Request struct {
	// Scope is one file or one directory, relative to the workspace
	// root.
	Scope    source.Path
	Language source.Language
	// Preferred is the weakest evidence the caller wants. An engine
	// answering below it is reported as [trust.Degraded] rather than
	// refused, because a weaker answer with its tier stated is worth
	// more than nothing.
	Preferred trust.Fidelity
}

// Query is what to search for.
//
// Text matches declaration names fuzzily and documentation by content,
// so a caller that knows neither the exact name nor the unit can still
// find something. A caller that knows the name passes the name.
type Query struct {
	Text string
	// Kind narrows to one kind of declaration. The zero value matches
	// any.
	Kind sema.Kind
	// Private includes declarations not visible outside their unit.
	Private bool
	// Limit caps the items returned. Zero means the engine's default.
	Limit int
}
