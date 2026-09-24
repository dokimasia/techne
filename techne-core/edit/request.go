// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Request is a change a caller asks for.
type Request struct {
	Operation Operation
	// Scope is the file or directory in which the target is resolved.
	Scope source.Path
	// Language restricts the request to one language. Empty selects the
	// languages of Scope.
	Language source.Language
	Target   Target
	Args     Args
	// DryRun plans and checks the change without writing it.
	DryRun bool
}

// Outcome reports what the write path did or, for a dry run, would do.
// Applied is false and Changed is empty on every failure: a change is
// applied in full or not at all.
type Outcome struct {
	Operation Operation
	Status    trust.Status
	// Applied reports whether the write path wrote the change.
	Applied bool
	// Changed lists the files written. It is empty for a dry run and for a
	// change whose result equals the current content.
	Changed []source.Path
	// Changes are the changes written or, for a dry run, planned.
	Changes []Change
	// Rewrites are Changes rendered as text against the current files.
	Rewrites []Rewrite
	// Handle identifies the plan of a dry run for a later commit. It is empty
	// when the plan cannot be applied.
	Handle string
	// Diagnostics are the errors the gate found in the projected files when
	// it refused the change.
	Diagnostics []Finding
	// Provenance is the evidence of the engine that planned the change.
	Provenance trust.Provenance
	// Gate is the evidence of the engine that checked the change, or nil when
	// no engine checked it. A Syntactic gate verifies that the result parses.
	// A Resolved gate verifies that it type-checks.
	Gate *trust.Provenance
	// Reason explains a Refused or Unsupported status.
	Reason string
}

// Finding is a diagnostic from a gate, with the change that fixes it.
type Finding struct {
	Diagnostic diag.Diagnostic
	// Fix is the change that resolves the diagnostic. It is empty when there
	// is no fix or more than one plausible fix.
	Fix []Change
}

// Rewrite is one range a change replaces, with the text before and after
// the change.
type Rewrite struct {
	Path source.Path
	// Line is the one-based line on which the range starts.
	Line int
	// Was is the replaced text. It is empty for an insertion.
	Was string
	// Now is the replacement text.
	Now string
}
