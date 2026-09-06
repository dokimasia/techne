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

// merge combines what several languages answered about one scope, and
// says what none of them covered.
//
// A merged answer claims only the weakest evidence behind it. Half an
// answer from a parser makes the whole of it a parser's answer, because
// a caller trusting the tier would trust every item in the list.
//
// The engine field names each contributor. Naming one would say a
// different engine produced items it never saw.
//
// # A language that said nothing is not in parts
//
// It is in the declines, which is why they come in beside the answers.
// An engine that declined contributed no items and no provenance, so
// nothing in the merge is lowered by it, and the answer would claim
// total coverage of a scope it covered part of.
//
// Measured: relations over a TypeScript directory, with no language
// named, was answered by six servers for languages the directory holds
// none of, while both TypeScript engines declined. The answer was "0
// sites, resolved, total coverage, an empty answer here means there are
// none" — a negative claim made by the languages that were not there,
// about the one that was.
func merge[T any](
	parts []engine.Answer[T],
	want trust.Fidelity,
	silent engine.Declined,
) engine.Answer[T] {
	if len(parts) == 0 {
		return engine.Answer[T]{}
	}

	merged := parts[0]
	if len(parts) > 1 {
		merged = combined(parts, want)
	}
	if len(silent) == 0 {
		return merged
	}

	// Nothing answered for a language that could have. That is a gap in
	// the scope rather than a fact about it, so the answer stops
	// claiming to cover the whole and says which language is missing.
	merged.Provenance.Completeness = trust.ScopePartial
	merged.Provenance.Caveats = keep(merged.Provenance.Caveats, []trust.Caveat{{
		Code: trust.CaveatUnsupported,
		Note: "nothing answered for part of the scope: " + silent.Reason(),
	}})
	merged.Status = statused(merged.Provenance, want)
	return merged
}

// combined folds several languages' answers into one.
func combined[T any](parts []engine.Answer[T], want trust.Fidelity) engine.Answer[T] {
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

	merged.Status = statused(merged.Provenance, want)
	return merged
}

// statused is what an answer's own evidence makes it, so a merged answer
// says the same thing a single-engine answer at this tier would.
func statused(p trust.Provenance, want trust.Fidelity) trust.Status {
	switch {
	case p.Fidelity < want:
		return trust.Degraded
	case p.Completeness == trust.ScopePartial:
		return trust.Partial
	default:
		return trust.OK
	}
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
