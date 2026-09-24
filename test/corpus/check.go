// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus

import (
	"bytes"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/tool"
)

// Spans returns a problem for each declaration of items that disagrees with
// content, the file that the declarations are in, and for each of their
// members. Each declaration has a span, as the outline tool returns it at the
// detail source. A declaration disagrees in these cases:
//
//   - It has no span, or its span lies outside the file.
//   - Its snippet differs from the bytes of its span, or lacks its name.
//   - Its line, or the line or column of its span, differs from the start
//     offset of its span.
//   - Its span leaves the span of the declaration that contains it.
func Spans(content []byte, items []tool.Declaration) []string {
	var out []string
	var visit func(d tool.Declaration, within *source.Span)
	visit = func(d tool.Declaration, within *source.Span) {
		s := d.Span
		switch {
		case s == nil:
			out = append(out, fmt.Sprintf("%s has no span", d.Name))
			return
		case s.Start.Offset < 0 || s.Start.Offset > s.End.Offset || s.End.Offset > len(content):
			out = append(out, fmt.Sprintf("%s spans bytes %d to %d of a file of %d bytes",
				d.Name, s.Start.Offset, s.End.Offset, len(content)))
			return
		}
		if d.Snippet != string(content[s.Start.Offset:s.End.Offset]) {
			out = append(out, fmt.Sprintf("the snippet of %s differs from the bytes of its span", d.Name))
		}
		if !strings.Contains(d.Snippet, d.Name) {
			out = append(out, fmt.Sprintf("the snippet of %s lacks its name", d.Name))
		}
		line, column := Position(content, s.Start.Offset)
		if s.Start.Line != line || s.Start.Column != column || d.Line != line+1 {
			out = append(out, fmt.Sprintf("%s is at line %d, column %d of its span and line %d of the item,"+
				" and its start offset is at line %d, column %d", d.Name, s.Start.Line, s.Start.Column, d.Line,
				line, column))
		}
		if within != nil && (s.Start.Offset < within.Start.Offset || s.End.Offset > within.End.Offset) {
			out = append(out, fmt.Sprintf("%s leaves the span of the declaration that contains it", d.Name))
		}
		for _, member := range d.Members {
			visit(member, s)
		}
	}
	for _, d := range items {
		visit(d, nil)
	}
	return out
}

// Position returns the zero-based line and byte column of offset in
// content.
func Position(content []byte, offset int) (line, column int) {
	before := content[:offset]
	line = bytes.Count(before, []byte("\n"))
	return line, offset - (bytes.LastIndexByte(before, '\n') + 1)
}

// Ranked returns a problem when items does not start with a declaration
// named text.
func Ranked(text string, items []tool.Declaration) []string {
	switch {
	case len(items) == 0:
		return []string{fmt.Sprintf("the search for %s returned nothing", text)}
	case items[0].Name != text:
		return []string{fmt.Sprintf("the search for %s returned %s first", text, items[0].Name)}
	}
	return nil
}

// Sites returns a problem for each edge of items whose line shows neither
// name, nor the name at the far end of the edge, nor a receiver keyword such
// as this, and for each edge whose Via differs from its line. A receiver
// keyword refers to a type inside its own members, as this does in a static
// method of JavaScript and Self in an impl of Rust. read returns the content
// of a path of an answer.
func Sites(name string, items []tool.Connected, read func(string) ([]byte, error)) []string {
	var out []string
	for _, edge := range items {
		content, err := read(edge.Path)
		if err != nil {
			out = append(out, fmt.Sprintf("%s:%d: %v", edge.Path, edge.Line, err))
			continue
		}
		written, found := Line(content, edge.Line)
		switch {
		case !found:
			out = append(out, fmt.Sprintf("%s has no line %d", edge.Path, edge.Line))
		case !strings.Contains(written, name) && !strings.Contains(written, edge.Name) && !receiver(written):
			out = append(out, fmt.Sprintf("%s:%d shows neither %s nor %s", edge.Path, edge.Line, name, edge.Name))
		case edge.Via != "" && edge.Via != strings.TrimSpace(written):
			out = append(out, fmt.Sprintf("%s:%d reads %q, and the edge quotes %q",
				edge.Path, edge.Line, strings.TrimSpace(written), edge.Via))
		}
	}
	return out
}

// receiver reports whether line contains this, self, Self or super as a word.
func receiver(line string) bool {
	for _, keyword := range []string{"this", "self", "Self", "super"} {
		for at := strings.Index(line, keyword); at >= 0; {
			end := at + len(keyword)
			if !identifying(line, at-1) && !identifying(line, end) {
				return true
			}
			next := strings.Index(line[end:], keyword)
			if next < 0 {
				break
			}
			at = end + next
		}
	}
	return false
}

// identifying reports whether the byte at i of line belongs to an
// identifier.
func identifying(line string, i int) bool {
	if i < 0 || i >= len(line) {
		return false
	}
	b := line[i]
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// Line returns line n of content, counting from one, without its line
// ending, and false when content has fewer lines.
func Line(content []byte, n int) (string, bool) {
	lines := strings.Split(string(content), "\n")
	if n < 1 || n > len(lines) {
		return "", false
	}
	return strings.TrimSuffix(lines[n-1], "\r"), true
}
