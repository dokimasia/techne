// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query

import (
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// keep adds the caveats a caller has not already been given.
//
// Every language answering a directory carries the limits of its own
// tier, and five parsers say the same thing about it. Repeating one
// caveat per language spends a caller's context on no extra fact.
//
// A caveat naming paths is kept as it stands, because the paths are what
// it is about and two of them differing is two facts.
func keep(into, from []trust.Caveat) []trust.Caveat {
	for _, add := range from {
		seen := false
		for _, held := range into {
			seen = seen || (len(add.Paths) == 0 && len(held.Paths) == 0 &&
				held.Code == add.Code && held.Note == add.Note)
		}
		if !seen {
			into = append(into, add)
		}
	}
	return into
}

// merge combines what several languages answered about one scope.
//
// A merged answer claims only the weakest evidence behind it. Half an
// answer from a parser makes the whole of it a parser's answer, because
// a caller trusting the tier would trust every item in the list.
//
// The engine field names each contributor. Naming one would say a
// different engine produced items it never saw.
func merge[T any](parts []engine.Answer[T], want trust.Fidelity) engine.Answer[T] {
	if len(parts) == 0 {
		return engine.Answer[T]{}
	}
	if len(parts) == 1 {
		return parts[0]
	}

	merged := engine.Answer[T]{
		Provenance: trust.Provenance{
			Fidelity:     trust.Resolved,
			Completeness: trust.ScopeTotal,
		},
	}

	names := make([]string, 0, len(parts))
	for _, part := range parts {
		merged.Items = append(merged.Items, part.Items...)
		names = append(names, part.Provenance.Engine)
		merged.Provenance.Caveats = keep(merged.Provenance.Caveats, part.Provenance.Caveats)
		merged.Provenance.Fidelity = min(merged.Provenance.Fidelity, part.Provenance.Fidelity)
		merged.Provenance.Completeness = min(merged.Provenance.Completeness, part.Provenance.Completeness)
	}
	merged.Provenance.Engine = strings.Join(names, ", ")

	// The status is derived from the merged evidence, so it says the
	// same thing a single-engine answer at this tier would.
	switch {
	case merged.Provenance.Fidelity < want:
		merged.Status = trust.Degraded
	case merged.Provenance.Completeness == trust.ScopePartial:
		merged.Status = trust.Partial
	default:
		merged.Status = trust.OK
	}
	return merged
}
