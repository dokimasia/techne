// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust

// Fidelity says how an answer was bound.
//
// The ordering is load-bearing: the engine catalogue sorts by it and the
// write path compares against it, so a new tier takes its place in this
// sequence rather than being appended. The zero value is [None].
type Fidelity uint8

const (
	// None means nothing could answer.
	None Fidelity = iota
	// Syntactic means a parser matched text. A link that crosses a file
	// is name coincidence.
	Syntactic
	// Indexed means an index resolved the reference without a type
	// checker agreeing.
	Indexed
	// Resolved means a type checker bound the name to a declaration.
	Resolved
)

// Completeness says how much of the requested scope an engine examined.
//
// It is independent of [Fidelity]. The zero value is [ScopeUnknown], so
// an engine that says nothing is treated as having claimed nothing.
//
// The values carry a Scope prefix because [Status] also has a Partial
// and both live in this package. Status is named on every answer, so it
// keeps the bare names.
type Completeness uint8

const (
	// ScopeUnknown means the engine cannot say what it covered.
	ScopeUnknown Completeness = iota
	// ScopePartial means the engine knows it missed files and names them
	// in the caveats.
	ScopePartial
	// ScopeTotal means the engine examined every file in the requested
	// scope.
	ScopeTotal
)

// fidelityNames is the single definition point for the wire form of
// each tier. An answer carries these strings to a caller.
var fidelityNames = map[Fidelity]string{
	None:      "none",
	Syntactic: "syntactic",
	Indexed:   "indexed",
	Resolved:  "resolved",
}

// String returns the wire form of the tier, or that of [None] for a
// value outside the set.
func (f Fidelity) String() string {
	if name, ok := fidelityNames[f]; ok {
		return name
	}
	return fidelityNames[None]
}

// completenessNames is the single definition point for the wire form of
// each coverage claim.
var completenessNames = map[Completeness]string{
	ScopeUnknown: "unknown",
	ScopePartial: "partial",
	ScopeTotal:   "total",
}

// String returns the wire form of the coverage claim, or that of
// [ScopeUnknown] for a value outside the set.
func (c Completeness) String() string {
	if name, ok := completenessNames[c]; ok {
		return name
	}
	return completenessNames[ScopeUnknown]
}

// SupportsNegativeClaim reports whether an empty answer means there are
// none, rather than that none were found.
//
// It requires both [Resolved] binding and [ScopeTotal] coverage. Either
// alone is a trap: a parser that read every file still matches on name
// coincidence, and a type checker that saw half the workspace found half
// the references.
func SupportsNegativeClaim(f Fidelity, c Completeness) bool {
	return f == Resolved && c == ScopeTotal
}
