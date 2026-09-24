// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import "go.dokimi.dev/techne/core/trust"

// Result is the return value of every engine port. It has no field for the
// engine's name or tier, and Lowered can only lower the tier the engine
// declares, so an engine cannot overstate its evidence. Services turn a
// Result into an Answer with Publish.
type Result[T any] struct {
	Items []T

	// Completeness is how much of the requested scope the engine examined.
	Completeness trust.Completeness

	// Caveats are limits on this answer, such as files the engine did not
	// read.
	Caveats []trust.Caveat

	// Lowered is a tier below the engine's declared one, for an answer worth
	// less than usual, such as a type check of code that does not compile.
	// Publish uses the lower of the two. The zero value keeps the declared
	// tier.
	Lowered trust.Fidelity

	// Skipped reports that the scope contains no file of the engine's
	// language. Services leave skipped answers out of the evidence they merge
	// across languages. An engine that read files without a match returns an
	// empty result with Skipped false. An engine that cannot find what the
	// request names returns ErrDecline.
	Skipped bool
}

// Publish stamps r with the evidence behind it. The engine name and tier
// come from e, never from r. The status is Degraded when the tier is below
// want, Partial when the coverage is partial, and OK otherwise. Degraded
// takes precedence over Partial.
//
// Services call Publish. Engines never do.
func Publish[T any](r Result[T], e Engine, role Role, want trust.Fidelity) Answer[T] {
	held := e.Fidelity(role)
	if r.Lowered != trust.None && r.Lowered < held {
		held = r.Lowered
	}

	status := trust.OK
	switch {
	case held < want:
		status = trust.Degraded
	case r.Completeness == trust.ScopePartial:
		status = trust.Partial
	}

	return Answer[T]{
		Items:   r.Items,
		Status:  status,
		Skipped: r.Skipped,
		Provenance: trust.Provenance{
			Engine:       e.Name(),
			Fidelity:     held,
			Completeness: r.Completeness,
			Caveats:      r.Caveats,
		},
	}
}
