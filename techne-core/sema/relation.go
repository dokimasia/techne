// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import (
	"go.dokimi.dev/techne/core/internal/wire"
	"go.dokimi.dev/techne/core/source"
)

// RelationKind is the direction of an edge between two declarations.
// [RelationKind.Inverse] returns the opposite direction. The zero value is
// RelationUnknown.
type RelationKind uint8

const (
	// RelationUnknown is an unclassified edge. It has no inverse.
	RelationUnknown RelationKind = iota
	// Calls leads from a callable to the callables it calls.
	Calls
	// CalledBy leads from a callable to its callers.
	CalledBy
	// Implements leads from a type to the interfaces it satisfies.
	Implements
	// ImplementedBy leads from an interface to the types that satisfy it.
	ImplementedBy
	// References leads from a declaration to the declarations it refers to.
	References
	// ReferencedBy leads from a declaration to the declarations that refer
	// to it.
	ReferencedBy
	// Imports leads from a file to the names it imports.
	Imports
	// ImportedBy leads from a name to the files that import it.
	ImportedBy
	// Embeds leads from a type to the types it embeds or extends.
	Embeds
	// EmbeddedBy leads from a type to the types that embed or extend it.
	EmbeddedBy
)

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

var relationWords = wire.New(RelationUnknown, map[RelationKind]string{
	RelationUnknown: "unknown",
	Calls:           "calls",
	CalledBy:        "called-by",
	Implements:      "implements",
	ImplementedBy:   "implemented-by",
	References:      "references",
	ReferencedBy:    "referenced-by",
	Imports:         "imports",
	ImportedBy:      "imported-by",
	Embeds:          "embeds",
	EmbeddedBy:      "embedded-by",
})

// RelationKinds returns every RelationKind except RelationUnknown, each next
// to its inverse.
func RelationKinds() []RelationKind {
	return []RelationKind{
		Calls, CalledBy,
		Implements, ImplementedBy,
		References, ReferencedBy,
		Imports, ImportedBy,
		Embeds, EmbeddedBy,
	}
}

// Inverse returns the kind that follows the same edges in the opposite
// direction, or RelationUnknown if r has no inverse.
func (r RelationKind) Inverse() RelationKind { return inverses[r] }

// String returns the wire string of r, or "unknown" if r is not a declared
// RelationKind.
func (r RelationKind) String() string { return relationWords.String(r) }

// MarshalJSON encodes r as its wire string.
func (r RelationKind) MarshalJSON() ([]byte, error) { return relationWords.Marshal(r) }

// UnmarshalJSON decodes a wire string. An unknown string decodes to
// RelationUnknown.
func (r *RelationKind) UnmarshalJSON(b []byte) error { return relationWords.Unmarshal(b, r) }

// Relation is one edge from the declaration a question is about. It records
// the far end only, because the near end is the same for every edge in an
// answer.
type Relation struct {
	// Kind is the direction the caller asked for.
	Kind RelationKind `json:"kind"`
	// To is the declaration at the far end of the edge.
	To Symbol `json:"to"`
	// At is the source range of the call or reference.
	At source.Span `json:"at"`
	// Via is the trimmed source line that At starts on.
	Via string `json:"via,omitempty"`
}
