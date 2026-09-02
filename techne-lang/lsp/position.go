// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"fmt"
	"os"
	"strings"

	"go.dokimi.dev/techne/core/source"
	"go.lsp.dev/protocol"
)

// document is a file's bytes and where each of its lines starts.
//
// Every role converts between the two coordinate systems, and both
// conversions need the file: the protocol names a line and a character
// and techne names a byte offset, and neither can be worked out from the
// other without the text between them.
//
// It is read once per file per call and passed around, because a scope
// holding fifty files would otherwise read each of them once per symbol
// found in it.
type document struct {
	path    source.Path
	content []byte
	// at is the offset each line begins at, so a line number indexes
	// straight into the file.
	at []int
}

// read loads a file for one call.
func (e *Engine) read(p source.Path) (document, error) {
	content, err := os.ReadFile(e.fullPath(p))
	if err != nil {
		return document{}, fmt.Errorf("lsp: read %s: %w", p, err)
	}
	return document{path: p, content: content, at: lines(content)}, nil
}

// lines is where each line of a file starts.
func lines(content []byte) []int {
	at := []int{0}
	for i, b := range content {
		if b == '\n' {
			at = append(at, i+1)
		}
	}
	return at
}

// line is one line of the file, without its terminator.
func (d document) line(n uint32) string {
	if int(n) >= len(d.at) {
		return ""
	}
	end := len(d.content)
	if int(n)+1 < len(d.at) {
		end = d.at[n+1] - 1
	}
	return string(d.content[d.at[n]:min(end, len(d.content))])
}

// span turns a protocol range into one this vocabulary counts in.
func (d document) span(held protocol.Range) source.Span {
	return source.Span{
		Path:  d.path,
		Start: d.position(held.Start),
		End:   d.position(held.End),
	}
}

// position turns a protocol position into one this vocabulary counts in.
//
// The protocol measures a character offset in UTF-16 code units unless a
// client negotiates otherwise, and techne measures bytes. On a line
// holding anything outside ASCII the two differ, and a span taken as
// bytes lands in the middle of a rune: the snippet cut from it is
// mangled and the edit computed from it writes over half a character.
//
// A line past the end of the file keeps the coordinates it arrived with
// and gets no offset. There is nothing to count bytes in, and a zero
// offset would name the start of the file.
func (d document) position(held protocol.Position) source.Position {
	if int(held.Line) >= len(d.at) {
		return source.Position{Line: int(held.Line), Column: int(held.Character)}
	}
	column := bytesFor(d.line(held.Line), held.Character)
	return source.Position{
		Offset: d.at[held.Line] + column,
		Line:   int(held.Line),
		Column: column,
	}
}

// mark is the reverse: a position this vocabulary counts in, as the
// protocol writes one.
//
// A caller may hand over a line and a column with no offset, because it
// is looking at an editor rather than at a byte count. It may equally
// hand over an offset alone, because it read the position out of an
// answer this package gave it. Both are accepted, and the offset wins
// where a caller sent both and they disagree — it is the coordinate
// nothing can round.
func (d document) mark(at source.Position) protocol.Position {
	line, column := at.Line, at.Column
	if at.Offset > 0 {
		line, column = d.lineAt(at.Offset)
	}
	if line < 0 || line >= len(d.at) {
		return protocol.Position{}
	}
	return protocol.Position{
		Line:      uint32(line),
		Character: unitsFor(d.line(uint32(line)), column),
	}
}

// lineAt is which line an offset falls on, and how far into it.
func (d document) lineAt(offset int) (line, column int) {
	if offset < 0 {
		return 0, 0
	}
	line = len(d.at) - 1
	for i, start := range d.at {
		if start > offset {
			line = i - 1
			break
		}
	}
	return line, offset - d.at[line]
}

// text is the source a span covers, and the empty string for a span this
// file does not hold.
func (d document) text(s source.Span) string {
	from, to := s.Start.Offset, s.End.Offset
	if from < 0 || to > len(d.content) || from >= to {
		return ""
	}
	return string(d.content[from:to])
}

// sourceLine is the line a span starts on, trimmed, which is what a
// caller reading a list of reference sites wants beside each one.
func (d document) sourceLine(s source.Span) string {
	return strings.TrimSpace(d.line(uint32(max(s.Start.Line, 0))))
}

// bytesFor is how many bytes of a line make up that many UTF-16 units.
//
// A rune outside the basic plane is two units and up to four bytes, so
// neither count stands in for the other. An emoji in a comment is enough
// to move every span after it on the line.
func bytesFor(line string, units uint32) int {
	if units == 0 {
		return 0
	}
	var held uint32
	for i, r := range line {
		if held >= units {
			return i
		}
		held++
		if r > 0xFFFF {
			held++
		}
	}
	return len(line)
}

// unitsFor is the reverse: how many UTF-16 units of a line make up that
// many bytes.
//
// A position handed to a server has to be written in the protocol's own
// coordinates. Without this, every request naming a position in a file
// holding one character outside ASCII asks about the wrong column, and
// the answer is about whatever is there instead.
func unitsFor(line string, bytes int) uint32 {
	if bytes <= 0 {
		return 0
	}
	var held uint32
	for i, r := range line {
		if i >= bytes {
			return held
		}
		held++
		if r > 0xFFFF {
			held++
		}
	}
	return held
}
