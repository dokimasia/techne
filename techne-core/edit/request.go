// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Request is one change a caller asked for.
//
// It is vocabulary rather than a service's own type, so a tool can name
// a change without depending on the thing that applies one.
type Request struct {
	Operation Operation
	// Scope is the file or directory the target is looked for in.
	Scope    source.Path
	Language source.Language
	Target   Target
	Args     Args
	// DryRun computes and gates the change without writing it.
	DryRun bool
}

// Outcome is what the write path did, or would have done.
//
// Applied is false and Changed is empty on every failure path. There is
// no partial success: either the change passed the gate and the files
// are as the plan described, or every file is as it was.
//
// It is named for what it is rather than called a result, because core
// already has two of those and they are different things: an engine
// returns what it found, a tool returns what it encoded, and this says
// what happened to the workspace.
type Outcome struct {
	Operation Operation
	Status    trust.Status
	Applied   bool
	// Changed names the files written, and is empty for a dry run.
	Changed []source.Path
	// Changes is what was done, or what a dry run would do.
	Changes []Change
	// Rewrites is the same thing read back against the files, so a
	// caller reviews text rather than byte ranges.
	Rewrites []Rewrite
	// Handle fetches this plan back to apply it, and is present on a
	// preview that could be applied. It is a name rather than the plan
	// because the plan goes to a caller and comes back, and a caller
	// that had to reproduce it exactly would sometimes not.
	Handle string
	// Diagnostics are what the gate reported, and are the reason a
	// change that planned cleanly was not applied. Each carries the
	// change that resolves it where there is one obvious change.
	Diagnostics []Finding
	Provenance  trust.Provenance
	// Reason says why, when the status is refused or unsupported.
	Reason string
}

// Finding is one thing a gate reported, and the change that resolves it.
//
// It is here rather than beside [go.dokimi.dev/techne/core/diag.Diagnostic]
// because a diagnostic is a fact about code and a fix is a change to it,
// and the package holding the first cannot name the second without the
// two importing each other. This one already names both.
//
// Pairing them is what turns a lint, fix and verify cycle into two round
// trips rather than five: a caller never works the edit out from the
// message.
type Finding struct {
	Diagnostic diag.Diagnostic
	// Fix is the change that resolves it, and is empty where there is
	// none or where there is more than one plausible one. Three payloads
	// to save a round trip that may not be taken is a bad trade.
	Fix []Change
}

// Rewrite is one range a change replaces, with the text on both sides.
//
// A plan says which bytes move. A caller reviewing a change wants to
// read what goes and what arrives, and the write path is where both are
// known: it has read the file and the planner that produced the offsets
// has not.
type Rewrite struct {
	Path source.Path
	// Line is where the range starts, counting from one, so it reads as
	// an editor reports it.
	Line int
	// Was is the text the change replaces, empty for an insertion.
	Was string
	// Now is the text that takes its place.
	Now string
}
