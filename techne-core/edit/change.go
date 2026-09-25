// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"bytes"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// The line endings that [Apply] writes.
const (
	lf   = "\n"
	crlf = "\r\n"
)

// ChangeKind is what a Change does to its path. The zero value is
// ChangeUnset.
type ChangeKind uint8

const (
	// ChangeUnset is a change without a kind. Applying one is a programming
	// error.
	ChangeUnset ChangeKind = iota
	// ChangeEdit rewrites byte ranges of an existing file.
	ChangeEdit
	// ChangeCreate writes a new file.
	ChangeCreate
	// ChangeDelete removes a file.
	ChangeDelete
	// ChangeMove moves a file to Change.To. Edits to the same path in the
	// same plan apply before the move.
	ChangeMove
)

// TextEdit replaces one byte range with New. An empty range inserts New.
type TextEdit struct {
	Span source.Span
	New  string
}

// Change is what one operation does to one path. Only the field that
// matches Kind is set: Edits for ChangeEdit, Content for ChangeCreate, and To
// for ChangeMove. Building a Change never touches disk.
type Change struct {
	Kind ChangeKind
	Path source.Path
	// To is the destination of a ChangeMove.
	To source.Path
	// Edits are the edits of a ChangeEdit, in the order Apply requires.
	Edits []TextEdit
	// Content is the body of a ChangeCreate.
	Content []byte
}

// Apply applies edits to content in one pass and returns the result. Each
// edit starts at or after the end of the previous edit. Edits that share a
// start offset apply in list order, as LSP 3.17 specifies for text edits:
// inserts at one offset keep their order, and an insert can precede a
// replacement at the same offset.
//
// The new text of each edit takes the line ending of content, as an editor
// writes an edit into its buffer: \r\n when more than half of the line
// endings of content are \r\n, and \n otherwise. The new text is unchanged
// for content without a line ending. A \r without a \n after it ends no
// line, and Apply leaves it as it is.
//
// Apply returns an error wrapping ErrDisordered for edits out of order, and
// an error for an edit that ends past the end of content. Planners and the
// write path both call Apply, so a preview and the applied change contain
// the same bytes.
func Apply(content []byte, edits []TextEdit) ([]byte, error) {
	if err := ordered(edits); err != nil {
		return nil, err
	}
	ending := endingOf(content)
	var out bytes.Buffer
	out.Grow(len(content))
	at := 0
	for i, e := range edits {
		start, end := e.Span.Start.Offset, e.Span.End.Offset
		if end > len(content) {
			return nil, fmt.Errorf("edit: edit %d ends at byte %d of a %d-byte file", i, end, len(content))
		}
		out.Write(content[at:start])
		out.WriteString(ended(e.New, ending))
		at = end
	}
	out.Write(content[at:])
	return out.Bytes(), nil
}

// endingOf returns the line ending of content: crlf when more than half of its line endings are
// crlf, lf when they are not, and the empty string for content without a line ending.
func endingOf(content []byte) string {
	lines := bytes.Count(content, []byte(lf))
	switch {
	case lines == 0:
		return ""
	case 2*bytes.Count(content, []byte(crlf)) > lines:
		return crlf
	}
	return lf
}

// ended returns text with each of its line endings written as ending. An empty ending leaves
// text unchanged.
func ended(text, ending string) string {
	if ending == "" || !strings.Contains(text, lf) {
		return text
	}
	unified := strings.ReplaceAll(text, crlf, lf)
	if ending == lf {
		return unified
	}
	return strings.ReplaceAll(unified, lf, crlf)
}

// ordered returns an error wrapping ErrDisordered unless every edit ends at
// or after its start and starts at or after the end of the previous edit.
func ordered(edits []TextEdit) error {
	end := 0
	for i, e := range edits {
		start := e.Span.Start.Offset
		switch {
		case e.Span.End.Offset < start:
			return fmt.Errorf("%w: edit %d ends before it starts", ErrDisordered, i)
		case start < end:
			return fmt.Errorf("%w: edit %d starts at byte %d, before byte %d", ErrDisordered, i, start, end)
		}
		end = e.Span.End.Offset
	}
	return nil
}
