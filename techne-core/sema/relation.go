// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "go.dokimi.dev/techne/core/source"

// RelationKind is how one declaration reaches another.
//
// Every directed kind has an inverse, so an engine implements whichever
// direction it can compute and a service asks for the direction the
// caller wanted. The zero value is [RelationUnknown], which has no
// inverse.
type RelationKind uint8

const (
	// RelationUnknown means the edge was not classified.
	RelationUnknown RelationKind = iota
	Calls
	CalledBy
	Implements
	ImplementedBy
	References
	ReferencedBy
	Imports
	ImportedBy
	Embeds
	EmbeddedBy
)

// inverses is the single definition point for the pairing. Every kind in
// [RelationKinds] appears exactly twice, once on each side.
var inverses = map[RelationKind]RelationKind{
	Calls:         CalledBy,
	CalledBy:      Calls,
	Implements:    ImplementedBy,
	ImplementedBy: Implements,
	References:    ReferencedBy,
	ReferencedBy:  References,
	Imports:       ImportedBy,
	ImportedBy:    Imports,
	Embeds:        EmbeddedBy,
	EmbeddedBy:    Embeds,
}

// RelationKinds returns every directed kind, excluding
// [RelationUnknown].
//
// It is the list the pairing is checked against, so a kind added without
// an inverse fails this package's tests rather than reaching a service
// that cannot turn the question around.
func RelationKinds() []RelationKind {
	return []RelationKind{
		Calls, CalledBy,
		Implements, ImplementedBy,
		References, ReferencedBy,
		Imports, ImportedBy,
		Embeds, EmbeddedBy,
	}
}

// Inverse returns the kind that asks the same question from the other
// end, and [RelationUnknown] for a kind with no pairing.
func (r RelationKind) Inverse() RelationKind {
	return inverses[r]
}

// Relation is one edge and where in the source it was found.
type Relation struct {
	Kind RelationKind
	From ID
	To   ID
	// At is where the edge was written, which is the reference site
	// rather than either declaration.
	At source.Span
}
