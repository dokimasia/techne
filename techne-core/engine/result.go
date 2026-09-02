// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import "go.dokimi.dev/techne/core/trust"

// Result is what an engine returns.
//
// It carries what the engine found and the limits only the engine knows.
// It carries nothing the engine could use to overstate itself: there is
// no field for the engine's name or its tier, so an adapter cannot claim
// evidence it does not hold. That is the same reason a role is declined
// by lacking a method rather than by returning an error.
//
// A service turns one into an [Answer] with [Publish].
type Result[T any] struct {
	Items []T

	// Completeness is how much of the requested scope the engine
	// examined. Only the engine knows, so this is the one judgement it
	// makes about its own answer.
	Completeness trust.Completeness

	// Caveats are limits on this answer that a tier cannot express: what
	// drifted, what was truncated, what no static analysis sees.
	Caveats []trust.Caveat

	// Skipped reports that the scope held no file this engine reads.
	//
	// It is a different answer from reading a scope and finding nothing
	// in it. The second says there are none, and is worth exactly what
	// the engine's tier is worth. The first says nothing at all: a
	// directory with no Ruby in it tells you nothing about Ruby, and a
	// parser saying so must not lower the evidence of a type checker
	// that did read the files beside it.
	//
	// The zero value counts, so an engine that does not set this is
	// merged as it always was. Setting it wrongly costs an answer its
	// say; leaving it unset costs nothing but the precision this exists
	// for.
	Skipped bool
}

// Publish stamps a result with the evidence behind it.
//
// The engine's name and tier are read from the engine rather than taken
// from the result. The status is derived here and nowhere else, so what
// counts as degraded cannot drift between one role and another.
//
// A published answer has run, so its status always reports a payload.
// When the tier is below the caller's floor and the scope is also short,
// the status is [trust.Degraded]: the caller asked for evidence and did
// not get it, which changes what the answer is worth more than a named
// gap does, and the gap is still in the caveats.
//
// Services call this. Engines never do.
func Publish[T any](r Result[T], e Engine, role Role, want trust.Fidelity) Answer[T] {
	held := e.Fidelity(role)

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
