// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust

import "go.dokimi.dev/techne/core/internal/wire"

// Fidelity is how an answer was bound, from weakest to strongest. The
// catalogue sorts engines by it and the write path compares against it, so
// the order is part of the API. The zero value is None.
type Fidelity uint8

const (
	// None means nothing answered.
	None Fidelity = iota
	// Syntactic means a parser matched text. Links between files are name
	// matches.
	Syntactic
	// Indexed means an index resolved the reference without a type checker.
	Indexed
	// Resolved means a type checker bound the name to its declaration.
	Resolved
)

// Completeness is how much of the requested scope an engine examined. It is
// independent of Fidelity. The zero value is ScopeUnknown.
//
// The values have a Scope prefix because Status already declares Partial.
type Completeness uint8

const (
	// ScopeUnknown means the engine did not say what it covered.
	ScopeUnknown Completeness = iota
	// ScopePartial means the engine skipped files. Its caveats name them.
	ScopePartial
	// ScopeTotal means the engine examined every file in the scope.
	ScopeTotal
)

var fidelityWords = wire.New(None, map[Fidelity]string{
	None:      "none",
	Syntactic: "syntactic",
	Indexed:   "indexed",
	Resolved:  "resolved",
})

var completenessWords = wire.New(ScopeUnknown, map[Completeness]string{
	ScopeUnknown: "unknown",
	ScopePartial: "partial",
	ScopeTotal:   "total",
})

// String returns the wire string of f, or "none" if f is not a declared
// Fidelity.
func (f Fidelity) String() string { return fidelityWords.String(f) }

// Fidelities returns every Fidelity, weakest first.
func Fidelities() []Fidelity { return []Fidelity{None, Syntactic, Indexed, Resolved} }

// String returns the wire string of c, or "unknown" if c is not a declared
// Completeness.
func (c Completeness) String() string { return completenessWords.String(c) }

// Completenesses returns every Completeness, weakest first.
func Completenesses() []Completeness {
	return []Completeness{ScopeUnknown, ScopePartial, ScopeTotal}
}

// SupportsNegativeClaim reports whether an empty answer proves absence. It
// requires Resolved binding over ScopeTotal coverage: a parser matches names
// even when it reads every file, and a type checker that saw half the scope
// found at most half the references.
func SupportsNegativeClaim(f Fidelity, c Completeness) bool {
	return f == Resolved && c == ScopeTotal
}
