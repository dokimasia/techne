// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// TargetKind is what an operation can be pointed at.
type TargetKind uint8

const (
	TargetUnset TargetKind = iota
	TargetSymbol
	TargetFile
	TargetSpan
)

// Target says what an operation is pointed at. Exactly one field
// matching Kind carries a value; the rest are zero.
//
// The zero Target has kind [TargetUnset] and points at nothing, so a
// request that forgot to name a target is refused rather than served
// against whatever the zero values happen to mean.
type Target struct {
	Kind   TargetKind
	Symbol sema.ID
	Path   source.Path
	Span   source.Span
}
