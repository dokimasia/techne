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

	// Only the engines that read something decide what the answer is
	// worth. A directory with no Ruby in it tells you nothing about
	// Ruby, and letting a parser that read no file lower the tier of a
	// type checker beside it reports resolved evidence as text-matched
	// and withdraws a negative claim the caller had earned.
	//
	// Reading a scope and finding nothing is a different answer, and it
	// counts: an engine that searched forty files and matched none still
	// cannot say there are no others, and its silence is what stops the
	// merged answer claiming there are.
	read := spoke(parts)

	names := make([]string, 0, len(parts))
	for _, part := range parts {
		merged.Items = append(merged.Items, part.Items...)
		if part.Skipped && read {
			continue
		}
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

// spoke reports whether any engine read a file.
//
// Where none did, every answer is left in. A scope holding no source at
// all is one nothing examined, and reporting the strongest tier among
// engines that read nothing would claim evidence none of them gathered.
func spoke[T any](parts []engine.Answer[T]) bool {
	for _, part := range parts {
		if !part.Skipped {
			return true
		}
	}
	return false
}
