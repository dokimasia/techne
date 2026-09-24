// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// TargetKind is the kind of thing an operation applies to. The zero value is
// TargetUnset.
type TargetKind uint8

const (
	// TargetUnset is no target. No operation accepts it.
	TargetUnset TargetKind = iota
	// TargetSymbol is a declaration, named by Target.Symbol.
	TargetSymbol
	// TargetFile is a file, named by Target.Path.
	TargetFile
	// TargetSpan is a range of a file, named by Target.Span.
	TargetSpan
)

// Target is what an operation applies to. Kind selects the field that is
// set: Symbol, Path or Span. The other fields are zero.
type Target struct {
	Kind   TargetKind
	Symbol sema.ID
	Path   source.Path
	Span   source.Span
}
