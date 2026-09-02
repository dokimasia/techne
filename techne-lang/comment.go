// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import "strings"

// CommentStyle is how a language writes comments and documentation. No
// grammar states it, and both reading a declaration's documentation and
// writing one need it.
//
// A language has more than one documentation form, and they are not
// interchangeable. Rust has four: /// and /**...*/ document what
// follows, //! and /*!...*/ document the item they are written inside.
// Java has two, the traditional /**...*/ and the markdown ///. C# has
// /// and /**...*/, and C has whatever Doxygen reads. One token does not
// mean one thing across languages either: /// is documentation in Java,
// Rust, C and C#, and a compiler directive in TypeScript.
//
// [CommentStyle.Doc] therefore holds a list rather than a pair of
// fields, and each language states its own.
type CommentStyle struct {
	// Line begins a comment running to the end of the line, trailing
	// space included, as in "// " or "# ".
	Line string

	// BlockOpen and BlockClose delimit a comment that may span lines.
	// Python has neither: it has no block comment at all.
	BlockOpen, BlockClose string

	// Doc holds every form the language's own documentation tool reads,
	// preferred form first. [CommentStyle.Documents] writes the first;
	// [CommentStyle.Documentation] recognises any of them.
	Doc []DocStyle
}

// DocStyle is one documentation form.
//
// Python's docstring is here as well as the comment forms. It is a
// string literal rather than a comment, but it is what the language
// documents with, and a caller reading or writing documentation wants
// one answer rather than two mechanisms.
type DocStyle struct {
	// Open begins the form: "///", "/**", "//!", `"""`.
	Open string

	// Close ends it, and is empty for a form running to the end of the
	// line. A line form documents one line, so several consecutive ones
	// make one comment.
	Close string

	// Continuation prefixes the lines between Open and Close where the
	// form has one, as Java's " * ". It is stripped when read and
	// written when emitted.
	Continuation string

	// Inside reports whether the form documents the item it is written
	// inside rather than the declaration that follows it. Rust spells
	// that "//!" and "/*!"; Python's docstring is the same relationship,
	// written as the first statement in the body.
	Inside bool
}

// Documents returns the form to write documentation in: the language's
// preferred one, or its line comment where it states none.
func (c CommentStyle) Documents() DocStyle {
	if len(c.Doc) > 0 {
		return c.Doc[0]
	}
	return DocStyle{Open: c.Line}
}

// Documentation reports whether a comment is documentation, and returns
// its text with the delimiters and any continuation prefixes removed.
//
// The longest matching form wins, so a language declaring both "//" and
// "///" reads "/// x" as the second rather than the first.
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

// Document renders documentation in the language's preferred form, at
// one indentation.
//
// It is the other half of [CommentStyle.Documentation]: what that reads,
// this writes. Reading a comment and writing the text back returns the
// comment, which is what makes a tool that rewrites documentation leave
// the rest of the file alone.
//
// The result carries no trailing newline. Where the comment goes relative
// to the declaration is the caller's business, and a language whose
// documentation sits inside the body puts it somewhere a line above the
// declaration is wrong.
func (c CommentStyle) Document(text, indent string) string {
	return c.Documents().write(text, indent)
}

// write renders text in this form.
func (d DocStyle) write(text, indent string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
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

// lineForm writes one comment per line, as Go, Rust, C# and Ruby
// document with.
func (d DocStyle) lineForm(lines []string, indent string) string {
	// A form written as the language's plain line comment carries the
	// space that separates it from the text; one written as a
	// documentation marker does not.
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

// blockForm writes a delimited comment whose inner lines carry no
// marker, which is how a Python docstring is laid out: the summary on
// the opening line, the closing delimiter on its own.
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

// markedForm writes a delimited comment whose inner lines carry a
// marker, as Javadoc and its imitators do.
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
	// The closing delimiter aligns under the marker, so it is written at
	// whatever the continuation puts before it.
	return strings.Join(append(out, indent+aligned(d.Continuation)+d.Close), "\n")
}

// aligned returns the whitespace a continuation writes before its
// marker, which is what puts a closing delimiter under it.
func aligned(continuation string) string {
	return continuation[:len(continuation)-len(strings.TrimLeft(continuation, " \t"))]
}

// read strips one comment down to the text it documents with.
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
		// One space after the delimiter separates it from the text and
		// is not part of it. Anything beyond that is the author's
		// indentation, which a code block in the documentation depends
		// on, so it stays.
		//
		// A marker-less block form writes no delimiter on the lines
		// after the first, so there is no separating space to take off
		// them and taking one would eat the author's first level of
		// indentation.
		if !bare || i == 0 {
			lines[i] = strings.TrimPrefix(lines[i], " ")
		}
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	return strings.Join(bounded(lines), "\n")
}

// dedent removes the indentation a marker-less block form carries from
// the code it sits in.
//
// A form whose inner lines carry no marker carries that indentation
// instead: a Python docstring inside a method is indented to the body,
// on every line, and none of that is the author's. What every line
// shares goes and what one line has more of stays, so a code block
// inside the documentation keeps its shape.
//
// The first line is left alone. It is written against the opening
// delimiter rather than against the margin, so it carries no
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

// shared returns how many leading bytes two margins have in common.
func shared(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// unprefixed removes a continuation marker and the one space after it,
// leaving a line that does not carry the marker untouched.
func unprefixed(line, marker string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, marker) {
		return line
	}
	return strings.TrimPrefix(trimmed, marker)
}

// bounded drops the blank lines a block form leaves at each end, which
// are the delimiters' own lines rather than documentation.
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
