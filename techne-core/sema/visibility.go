// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "encoding/json"

// Visibility is whether a declaration can be named outside the unit that
// declares it.
//
// It is three-valued because an engine may be unable to tell. Go and
// Python spell visibility in the name, so a parser reads it. Java, Rust
// and TypeScript spell it as a modifier or a keyword, which a name
// carries nothing of, and a parser that guessed would report every
// declaration as public.
//
// The zero value is [VisibilityUnknown], so an engine that cannot tell
// says so rather than claiming the commoner answer.
type Visibility uint8

const (
	// VisibilityUnknown means the engine could not tell. It is not a
	// synonym for unexported: a caller filtering to public API must not
	// silently drop everything an engine was unsure about.
	VisibilityUnknown Visibility = iota
	// Unexported means the declaration cannot be named outside its unit.
	Unexported
	// Exported means it can.
	Exported
)

// visibilityNames is the single definition point for the wire form.
var visibilityNames = map[Visibility]string{
	VisibilityUnknown: "unknown",
	Unexported:        "unexported",
	Exported:          "exported",
}

// String returns the wire form, or that of [VisibilityUnknown] for a
// value outside the set.
func (v Visibility) String() string {
	if name, ok := visibilityNames[v]; ok {
		return name
	}
	return visibilityNames[VisibilityUnknown]
}

// MarshalJSON writes the wire form rather than the number, so a caller
// reads a visibility without a lookup table.
func (v Visibility) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}
