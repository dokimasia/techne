// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

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
	// ChangeMove moves a file to To. It carries whatever the rest of the
	// plan leaves in it, so a plan that both rewrites and moves one file
	// arrives at the destination rewritten.
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

// Apply rewrites content with an edit list, in a single pass.
//
// It lives here because two places need the same answer from it: a
// planner working out what a file would become, and the write path
// working out what to put on disk. Two implementations would let a
// preview promise something an apply does not deliver, which is the one
// failure a dry run exists to rule out.
//
// The edits must be sorted by where they start and must not meet, which
// is what [Change] documents and what lets the walk copy what lies
// between them and never look back. An edit outside the content it was
// computed against is an error rather than a truncated write: byte
// ranges over other bytes describe other code and usually still compile.
func Apply(content []byte, edits []TextEdit) ([]byte, error) {
	var out strings.Builder
	out.Grow(len(content))

	at := 0
	for _, e := range edits {
		start, end := e.Span.Start.Offset, e.Span.End.Offset
		if start < at || end > len(content) || end < start {
			return nil, fmt.Errorf(
				"the edit at %d..%d is outside the %d bytes it was computed against",
				start, end, len(content))
		}
		out.Write(content[at:start])
		out.WriteString(e.New)
		at = end
	}
	out.Write(content[at:])
	return []byte(out.String()), nil
}
