// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "go.dokimi.dev/techne/core/internal/wire"

// Visibility reports whether code outside a declaration's unit can refer to
// it.
//
// Go and Python encode visibility in the name, so a parser can read it. Java,
// Rust, and TypeScript use modifiers, and a parser that reads names alone
// reports VisibilityUnknown for them. The zero value is VisibilityUnknown.
type Visibility uint8

const (
	// VisibilityUnknown means the engine did not determine the visibility.
	// Filters for exported declarations keep such a declaration, because it
	// can be exported.
	VisibilityUnknown Visibility = iota
	// Unexported declarations cannot be referred to outside their unit.
	Unexported
	// Exported declarations can be referred to outside their unit.
	Exported
)

var visibilityWords = wire.New(VisibilityUnknown, map[Visibility]string{
	VisibilityUnknown: "unknown",
	Unexported:        "unexported",
	Exported:          "exported",
})

// String returns the wire string of v, or "unknown" if v is not a declared
// Visibility.
func (v Visibility) String() string { return visibilityWords.String(v) }

// MarshalJSON encodes v as its wire string.
func (v Visibility) MarshalJSON() ([]byte, error) { return visibilityWords.Marshal(v) }

// UnmarshalJSON decodes a wire string. An unknown string decodes to
// VisibilityUnknown.
func (v *Visibility) UnmarshalJSON(b []byte) error { return visibilityWords.Unmarshal(b, v) }

// Visibilities returns every Visibility, including VisibilityUnknown.
func Visibilities() []Visibility {
	return []Visibility{VisibilityUnknown, Unexported, Exported}
}
