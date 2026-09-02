// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"slices"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Plan is what an operation would do, and what stands behind it.
//
// It holds no filesystem handle, so it can be inspected, diffed and
// thrown away without touching the workspace. A dry run is the real call
// without the write.
//
// A planner produces the changes. The write path fills in the
// preconditions and the provenance: a planner that sealed its own would
// produce a plan that applies cleanly to a file it never read, and one
// that stamped its own provenance could claim evidence it does not hold.
type Plan struct {
	Operation     Operation
	Changes       []Change
	Preconditions []Precondition
	Provenance    trust.Provenance
}

// Precondition pins the content a plan was computed against.
//
// Byte ranges computed against different content describe something
// else, and the result usually still compiles, so nothing downstream
// catches it.
type Precondition struct {
	Path source.Path
	// Digest is the SHA-256 of the file as the plan was computed
	// against it.
	Digest [32]byte
}

// Paths returns every path the plan touches, sorted and without
// repeats.
//
// A move touches two: the file it leaves and the file it becomes. Both
// need a lock and both must be free to write, so both are here.
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

// Reads returns the paths whose current content the plan depends on.
//
// A created file has no content to depend on, so pinning one would
// refuse every plan that makes a file. Everything else is read before it
// is written and must be what the planner saw.
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
