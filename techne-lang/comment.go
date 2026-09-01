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

// read strips one comment down to the text it documents with.
func (d DocStyle) read(text string) string {
	body := strings.TrimPrefix(text, d.Open)
	if d.Close != "" {
		body = strings.TrimSuffix(strings.TrimRight(body, " \t\n\r"), d.Close)
	}

	lines := strings.Split(body, "\n")
	for i := range lines {
		if i > 0 && d.Continuation != "" {
			lines[i] = unprefixed(lines[i], strings.TrimSpace(d.Continuation))
		}
		// One space after the delimiter separates it from the text and
		// is not part of it. Anything beyond that is the author's
		// indentation, which a code block in the documentation depends
		// on, so it stays.
		lines[i] = strings.TrimRight(strings.TrimPrefix(lines[i], " "), " \t\r")
	}
	return strings.Join(bounded(lines), "\n")
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
