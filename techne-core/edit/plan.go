// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"slices"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Plan is the set of changes an operation makes, with the evidence behind
// them. A Plan has no file handle, so it can be inspected and discarded
// without touching the workspace.
//
// A planner sets Operation and Changes. The write path sets Preconditions
// and Provenance, so a planner cannot seal content it did not read or state
// its own evidence.
type Plan struct {
	Operation     Operation
	Changes       []Change
	Preconditions []Precondition
	Provenance    trust.Provenance
}

// Precondition records the content of one file that a plan was computed
// against. The write path does not write a file whose digest differs from
// its precondition.
type Precondition struct {
	Path source.Path
	// Digest is the SHA-256 of the file content.
	Digest [32]byte
}

// Paths returns every path the plan changes, sorted and without duplicates.
// A move contributes its source and its destination. The write path locks
// the paths in this order.
func (p Plan) Paths() []source.Path {
	out := make([]source.Path, 0, len(p.Changes))
	for _, c := range p.Changes {
		out = append(out, c.Path)
		if c.Kind == ChangeMove && c.To != "" {
			out = append(out, c.To)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Reads returns every path whose current content the plan depends on,
// sorted and without duplicates. These are the changed paths except the
// ones the plan creates.
func (p Plan) Reads() []source.Path {
	out := make([]source.Path, 0, len(p.Changes))
	for _, c := range p.Changes {
		if c.Kind == ChangeCreate {
			continue
		}
		out = append(out, c.Path)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
