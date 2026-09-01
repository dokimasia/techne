// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query

import (
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

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
		merged.Provenance.Caveats = append(merged.Provenance.Caveats, part.Provenance.Caveats...)
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
