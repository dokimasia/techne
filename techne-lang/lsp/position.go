// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"slices"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
)

// document is the content of one file and the offset at which each of its lines starts. It
// converts positions between the protocol and techne:
//
//   - The protocol counts a line and a character in UTF-16 code units, the LSP 3.17 default.
//   - techne counts a byte offset, and a line and a column in bytes.
//
// A line ends at "\n". A "\r" before the "\n" is not part of the line. Each ASCII byte is one
// code unit, so a conversion inside the ASCII bytes that start a line takes constant time. A
// conversion past them decodes the line from its first byte outside ASCII. No conversion copies
// the content.
type document struct {
	path    source.Path
	content []byte
	// at is the byte offset at which each line starts. The file has len(at) lines.
	at []int
	// plain is the number of ASCII bytes at the start of each line.
	plain []int
	// names is the position of the name of each declaration that the server reported,
	// keyed by the offset at which the declaration starts. It is empty for a document whose
	// symbols were not read.
	names map[int]protocol.Position
}

// texted returns the document of content at p, with no name recorded.
func texted(p source.Path, content []byte) document {
	d := document{path: p, content: content, at: lines(content), names: map[int]protocol.Position{}}
	d.plain = make([]int, len(d.at))
	for n := range d.at {
		start, end := d.bounds(n)
		ascii := start
		for ascii < end && content[ascii] < utf8.RuneSelf {
			ascii++
		}
		d.plain[n] = ascii - start
	}
	return d
}

// lines returns the offset at which each line of content starts. The first line starts at 0,
// and each "\n" starts another line.
func lines(content []byte) []int {
	at := []int{0}
	for i, b := range content {
		if b == '\n' {
			at = append(at, i+1)
		}
	}
	return at
}

// bounds returns the offsets at which line n starts and ends, without its line ending.
func (d document) bounds(n int) (start, end int) {
	start, end = d.at[n], len(d.content)
	if n+1 < len(d.at) {
		end = d.at[n+1] - 1
	}
	if end > start && d.content[end-1] == '\r' {
		end--
	}
	return start, end
}

// line returns line n without its line ending, or the empty string for a line past the end.
func (d document) line(n uint32) string {
	if int(n) >= len(d.at) {
		return ""
	}
	start, end := d.bounds(int(n))
	return string(d.content[start:end])
}

// span converts a protocol range to a [source.Span] of the document.
func (d document) span(r protocol.Range) source.Span {
	return source.Span{Path: d.path, Start: d.position(r.Start), End: d.position(r.End)}
}

// position converts a protocol position to a [source.Position].
//
// A character past the end of its line is the end of the line, as LSP 3.17 specifies.
// Character 0 of the line after the last line is the end of the file. Every other position
// past the end of the file converts to the end of the file with the line and character it
// named, and [document.inside] reports it.
func (d document) position(p protocol.Position) source.Position {
	if int(p.Line) >= len(d.at) {
		return source.Position{Offset: len(d.content), Line: int(p.Line), Column: int(p.Character)}
	}
	column := d.bytesFor(int(p.Line), p.Character)
	return source.Position{Offset: d.at[p.Line] + column, Line: int(p.Line), Column: column}
}

// inside reports whether both ends of r are in the document, where character 0 of the line
// after the last line counts as the end of the file.
func (d document) inside(r protocol.Range) bool {
	for _, p := range []protocol.Position{r.Start, r.End} {
		switch {
		case int(p.Line) < len(d.at):
		case int(p.Line) == len(d.at) && p.Character == 0:
		default:
			return false
		}
	}
	return true
}

// mark converts the offset of a [source.Position] to a protocol position, or its line and
// column for an offset of 0, which is what a caller that reads an editor sends. It converts a
// line past the end of the document to line 0, character 0.
func (d document) mark(at source.Position) protocol.Position {
	line, column := at.Line, at.Column
	if at.Offset > 0 {
		line, column = d.lineAt(at.Offset)
	}
	if line < 0 || line >= len(d.at) {
		return protocol.Position{}
	}
	return protocol.Position{Line: uint32(line), Character: d.unitsFor(line, column)}
}

// lineAt returns the line that contains the byte at offset and the column of the byte in the
// line. A negative offset is line 0, column 0.
func (d document) lineAt(offset int) (line, column int) {
	if offset < 0 {
		return 0, 0
	}
	line, starts := slices.BinarySearch(d.at, offset)
	if !starts {
		line--
	}
	return line, offset - d.at[line]
}

// text returns the source that s covers, or the empty string for a span outside the file.
func (d document) text(s source.Span) string {
	from, to := s.Start.Offset, s.End.Offset
	if from < 0 || to > len(d.content) || from >= to {
		return ""
	}
	return string(d.content[from:to])
}

// sourceLine returns the line that s starts on, cut by [lang.Excerpt] around the start of s,
// without leading and trailing white space. It returns the empty string for a line outside
// the document.
func (d document) sourceLine(s source.Span) string {
	if s.Start.Line < 0 || s.Start.Line >= len(d.at) {
		return ""
	}
	start, end := d.bounds(s.Start.Line)
	return strings.TrimSpace(lang.Excerpt(d.content[start:end], s.Start.Offset-start))
}

// bytesFor returns the number of bytes of line n that units UTF-16 code units cover, up to the
// length of the line. A rune outside the Basic Multilingual Plane is two code units and four
// bytes.
func (d document) bytesFor(n int, units uint32) int {
	start, end := d.bounds(n)
	if units <= uint32(d.plain[n]) {
		return int(units)
	}
	counted := uint32(d.plain[n])
	for i := start + d.plain[n]; i < end; {
		if counted >= units {
			return i - start
		}
		r, size := utf8.DecodeRune(d.content[i:end])
		counted++
		if r > 0xFFFF {
			counted++
		}
		i += size
	}
	return end - start
}

// unitsFor returns the number of UTF-16 code units that the first width bytes of line n
// cover, up to the length of the line.
func (d document) unitsFor(n, width int) uint32 {
	start, end := d.bounds(n)
	if width <= 0 {
		return 0
	}
	if width <= d.plain[n] {
		return uint32(width)
	}
	counted := uint32(d.plain[n])
	for i := start + d.plain[n]; i < end && i-start < width; {
		r, size := utf8.DecodeRune(d.content[i:end])
		counted++
		if r > 0xFFFF {
			counted++
		}
		i += size
	}
	return counted
}
