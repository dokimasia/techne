// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import "go.dokimi.dev/techne/core/source"

// ChangeKind is what a change does to one path.
type ChangeKind uint8

const (
	// ChangeUnset means nobody set a kind. Applying one is a programming
	// error, not a refusal.
	ChangeUnset ChangeKind = iota
	// ChangeEdit rewrites ranges within an existing file.
	ChangeEdit
	// ChangeCreate writes a file that did not exist.
	ChangeCreate
	// ChangeDelete removes a file.
	ChangeDelete
	// ChangeMove moves a file to To, carrying its content unchanged.
	ChangeMove
)

// TextEdit replaces one range with new text. An edit whose span is empty
// inserts at that point.
type TextEdit struct {
	Span source.Span
	New  string
}

// Change is what one operation does to one path.
//
// A planner produces these and is handed no filesystem, so nothing here
// touches disk. Only the field matching Kind carries a value: Edits for
// [ChangeEdit], Content for [ChangeCreate], To for [ChangeMove].
//
// Edits are sorted by their span's start offset and do not overlap. The
// write path relies on both to apply them in one pass.
type Change struct {
	Kind ChangeKind
	Path source.Path
	// To is the destination of a [ChangeMove].
	To source.Path
	// Edits are the ranges a [ChangeEdit] rewrites.
	Edits []TextEdit
	// Content is the whole body of a [ChangeCreate].
	Content []byte
}
