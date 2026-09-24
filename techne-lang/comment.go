// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import "strings"

// CommentStyle is the comment and documentation syntax of a language.
//
// A language can have more than one documentation form, and the forms are
// not interchangeable. Rust has four: /// and /** */ document the
// declaration that follows, and //! and /*! */ document the item that
// contains them. Java has /** */ and ///. One token can also mean different
// things: /// is documentation in Java, Rust, C and C#, and a compiler
// directive in TypeScript. Each language module lists its own forms in Doc.
type CommentStyle struct {
	// Line starts a comment that ends at the end of the line, including the
	// space after it, such as "// " or "# ".
	Line string

	// BlockOpen and BlockClose delimit a comment that can span lines. They
	// are empty for a language without block comments, such as Python.
	BlockOpen, BlockClose string

	// Doc lists every documentation form of the language, the preferred
	// form first. [CommentStyle.Document] writes the first form, and
	// [CommentStyle.Documentation] reads every form.
	Doc []DocStyle
}

// DocStyle is one documentation form. A Python docstring is a DocStyle
// although it is a string literal, because Python documents with it. A form
// with an empty Open matches no comment.
type DocStyle struct {
	// Open starts the form, such as "///", "/**", "//!" or `"""`.
	Open string

	// Close is the delimiter that ends the form, or empty for a form that
	// ends at the end of the line. Consecutive lines of a line form make one
	// comment.
	Close string

	// Continuation is the prefix of each line between Open and Close, such
	// as " * " in Java. Reading removes it and writing adds it.
	Continuation string

	// Inside reports whether the form documents the item that contains it,
	// such as Rust's //! or a Python docstring, rather than the declaration
	// that follows it.
	Inside bool

	// Element is the XML element that wraps the text of the form, such as
	// summary in the XML documentation of C#, or empty for a form without
	// markup. Writing puts text that does not start with a tag inside the
	// element, on lines of its own, and [CommentStyle.Unwrapped] removes the
	// element again.
	Element string
}

// Unwrapped returns doc, documentation that [CommentStyle.Documentation]
// read, without the Element of the preferred form when the element alone
// makes up doc. A doc with more markup keeps it, such as a summary followed
// by the description of a parameter.
func (c CommentStyle) Unwrapped(doc string) string {
	element := c.Documents().Element
	if element == "" {
		return doc
	}
	opening, closing := "<"+element+">", "</"+element+">"
	inner, opened := strings.CutPrefix(strings.TrimSpace(doc), opening)
	inner, closed := strings.CutSuffix(inner, closing)
	if !opened || !closed || strings.Contains(inner, closing) {
		return doc
	}
	return strings.Join(bounded(strings.Split(inner, "\n")), "\n")
}

// Documents returns the form to write documentation in: the first form of
// Doc, or a line comment when Doc is empty.
func (c CommentStyle) Documents() DocStyle {
	if len(c.Doc) > 0 {
		return c.Doc[0]
	}
	return DocStyle{Open: c.Line}
}

// Documentation reports whether text is a documentation comment, and
// returns its content without delimiters and continuation prefixes. When
// more than one form matches, the form with the longest Open applies, so
// "/// x" reads as "///" in a language that also declares "//".
func (c CommentStyle) Documentation(text string) (string, bool) {
	trimmed := strings.TrimLeft(text, " \t")

	best := -1
	for i, style := range c.Doc {
		if style.Open == "" || !strings.HasPrefix(trimmed, style.Open) {
			continue
		}
		if best < 0 || len(style.Open) > len(c.Doc[best].Open) {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}
	return c.Doc[best].read(trimmed), true
}

// Document returns text as a comment in the preferred form, with indent
// before each line and no trailing newline.
//
// Documentation reverses it. For a block form, Documentation of the result
// returns text. For a line form, Documentation of each line returns the
// corresponding line of text. For a form with an Element, Unwrapped of what
// Documentation returns is text.
func (c CommentStyle) Document(text, indent string) string {
	return c.Documents().write(text, indent)
}

// write returns text as a comment in this form.
func (d DocStyle) write(text, indent string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if d.Element != "" && !strings.HasPrefix(strings.TrimSpace(text), "<") {
		text = "<" + d.Element + ">\n" + strings.Trim(text, "\n") + "\n</" + d.Element + ">"
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	lines = bounded(lines)
	if len(lines) == 0 {
		lines = []string{""}
	}

	switch {
	case d.Close == "":
		return d.lineForm(lines, indent)
	case d.Continuation == "":
		return d.blockForm(lines, indent)
	default:
		return d.markedForm(lines, indent)
	}
}

// lineForm writes one line comment per line, as Go, Rust, C# and Ruby
// document. A blank line is the marker without a trailing space.
func (d DocStyle) lineForm(lines []string, indent string) string {
	open := d.Open
	if !strings.HasSuffix(open, " ") {
		open += " "
	}
	bare := strings.TrimRight(open, " ")

	out := make([]string, len(lines))
	for i, line := range lines {
		if line == "" {
			out[i] = indent + bare
			continue
		}
		out[i] = indent + open + line
	}
	return strings.Join(out, "\n")
}

// blockForm writes a delimited comment without a marker on its inner lines,
// as a Python docstring: the first line after the opening delimiter and the
// closing delimiter on a line of its own.
func (d DocStyle) blockForm(lines []string, indent string) string {
	if len(lines) == 1 {
		return indent + d.Open + lines[0] + d.Close
	}

	out := make([]string, 0, len(lines)+1)
	out = append(out, indent+d.Open+lines[0])
	for _, line := range lines[1:] {
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, indent+line)
	}
	return strings.Join(append(out, indent+d.Close), "\n")
}

// markedForm writes a delimited comment with a marker on each inner line,
// as Javadoc. The closing delimiter is aligned under the marker.
func (d DocStyle) markedForm(lines []string, indent string) string {
	out := make([]string, 0, len(lines)+2)
	out = append(out, indent+d.Open)
	for _, line := range lines {
		if line == "" {
			out = append(out, indent+strings.TrimRight(d.Continuation, " \t"))
			continue
		}
		out = append(out, indent+d.Continuation+line)
	}
	return strings.Join(append(out, indent+aligned(d.Continuation)+d.Close), "\n")
}

// aligned returns the whitespace before the marker of a continuation.
func aligned(continuation string) string {
	return continuation[:len(continuation)-len(strings.TrimLeft(continuation, " \t"))]
}

// read returns the content of one comment in this form.
//
// It removes the delimiters, the continuation markers, and the one space
// after a delimiter or marker. Further indentation belongs to the content,
// such as a code block, and read keeps it. The lines after the first of a
// form without markers have no delimiter, so read removes only the
// indentation they share.
func (d DocStyle) read(text string) string {
	body := strings.TrimPrefix(text, d.Open)
	if d.Close != "" {
		body = strings.TrimSuffix(strings.TrimRight(body, " \t\n\r"), d.Close)
	}

	lines := strings.Split(body, "\n")
	bare := d.Close != "" && d.Continuation == ""
	if bare {
		dedent(lines)
	}
	for i := range lines {
		if i > 0 && d.Continuation != "" {
			lines[i] = unprefixed(lines[i], strings.TrimSpace(d.Continuation))
		}
		if !bare || i == 0 {
			lines[i] = strings.TrimPrefix(lines[i], " ")
		}
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	return strings.Join(bounded(lines), "\n")
}

// dedent removes the indentation that every non-blank line after the first
// shares. The first line follows the opening delimiter, so it has no
// indentation to share.
func dedent(lines []string) {
	common := ""
	first := true
	for _, line := range lines[min(1, len(lines)):] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		margin := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if first {
			common, first = margin, false
			continue
		}
		common = common[:shared(common, margin)]
	}
	if common == "" {
		return
	}
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.TrimPrefix(lines[i], common)
	}
}

// shared returns the length of the common prefix of a and b.
func shared(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// unprefixed removes marker and the space after it from line. It returns a
// line without the marker unchanged.
func unprefixed(line, marker string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, marker) {
		return line
	}
	return strings.TrimPrefix(trimmed, marker)
}

// bounded removes the blank lines at both ends of lines.
func bounded(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}
