// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query

import (
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// keep appends to into the caveats of from that into does not contain. A caveat without
// paths is left out when into contains one with its code and note. A caveat with paths is
// always kept, because two answers name different files.
func keep(into, from []trust.Caveat) []trust.Caveat {
	for _, add := range from {
		seen := false
		for _, one := range into {
			seen = seen || (len(add.Paths) == 0 && len(one.Paths) == 0 &&
				one.Code == add.Code && one.Note == add.Note)
		}
		if !seen {
			into = append(into, add)
		}
	}
	return into
}

// merge combines the answers of the languages of one scope, of which parts contains at
// least one. It returns one answer unchanged, and [combined] of two or more. A language in
// silent, whose engines declined,
// makes the merged answer partial, with a [trust.CaveatUnsupported] caveat that contains
// their reasons.
func merge[T any](
	parts []engine.Answer[T],
	want trust.Fidelity,
	silent engine.Declined,
) engine.Answer[T] {
	merged := parts[0]
	if len(parts) > 1 {
		merged = combined(parts, want)
	}
	if len(silent) == 0 {
		return merged
	}

	merged.Provenance.Completeness = trust.ScopePartial
	merged.Provenance.Caveats = keep(merged.Provenance.Caveats, []trust.Caveat{{
		Code: trust.CaveatUnsupported,
		Note: "nothing answered for part of the scope: " + silent.Reason(),
	}})
	merged.Status = statused(merged.Provenance, want)
	return merged
}

// combined merges two or more answers: the items of every answer, and the weakest tier,
// the weakest completeness, the caveats and the engine names of the answers that read a
// file. A skipped answer did not read a file, so it counts only when every answer is
// skipped. A scope without a file of any language then claims the weakest tier of the
// engines asked.
func combined[T any](parts []engine.Answer[T], want trust.Fidelity) engine.Answer[T] {
	merged := engine.Answer[T]{
		Provenance: trust.Provenance{
			Fidelity:     trust.Resolved,
			Completeness: trust.ScopeTotal,
		},
	}
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

// statused returns the status of an answer with provenance p, by the rule of
// [engine.Publish]: degraded below want, partial for partial coverage, and OK otherwise.
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

// spoke reports whether an answer of parts read a file of the scope, which a skipped
// answer did not.
func spoke[T any](parts []engine.Answer[T]) bool {
	for _, part := range parts {
		if !part.Skipped {
			return true
		}
	}
	return false
}
