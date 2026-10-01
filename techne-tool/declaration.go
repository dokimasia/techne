// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"slices"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Narrow limits an answer to the declarations that a caller names. Over a whole file the
// levels [Docs] and [Source] cost more than the file, and over a few named declarations they
// cost a fraction of it.
type Narrow struct {
	// Names keeps the declarations of these names, each plain or qualified as Instant.Time.
	Names []string
	// Kind keeps the declarations of one kind. [sema.KindUnknown] keeps every kind.
	Kind sema.Kind
	// Prefix keeps the declarations whose names start with it.
	Prefix string
	// Private keeps the declarations that are not visible outside their unit.
	Private bool
}

// wanted reports whether d is a declaration that n keeps.
func (n Narrow) wanted(d Declaration) bool {
	if !n.Private && d.Visibility == sema.Unexported {
		return false
	}
	if n.Kind != sema.KindUnknown && d.Kind != n.Kind {
		return false
	}
	if n.Prefix != "" && !strings.HasPrefix(d.Name, n.Prefix) {
		return false
	}
	return len(n.Names) == 0 || slices.Contains(n.Names, d.Name) ||
		d.qualified != "" && slices.Contains(n.Names, d.qualified)
}

// Apply returns the declarations of items that n keeps. A declaration that n keeps comes with
// all its members. A declaration that n does not keep comes with the members that n keeps
// under it, and is left out when it has none.
func (n Narrow) Apply(items []Declaration) []Declaration {
	out := []Declaration{}
	for _, item := range items {
		if n.wanted(item) {
			out = append(out, item)
			continue
		}
		if kept := n.Apply(item.Members); len(kept) > 0 {
			item.Members = kept
			out = append(out, item)
		}
	}
	return out
}

// Members are the declarations that a declaration contains, in a type whose schema refers to
// the schema of a declaration, because a schema that contains itself does not terminate.
type Members []Declaration

// Declaration is one item of an answer: the fields of a [sema.Symbol] that the level of
// [Detail] selects. The facts of the whole answer are in [Scope].
type Declaration struct {
	Name string    `json:"name"`
	Kind sema.Kind `json:"kind"`
	// Line is the line on which the declaration starts, counted from one.
	Line int `json:"line"`
	// Path is the file of the declaration when it is not the file of the scope, as in an answer
	// about more than one file.
	Path string `json:"path,omitempty"`
	// Summary is the first sentence of the documentation comment, at [Summaries].
	Summary string `json:"summary,omitempty"`
	// Signature is the declaration without its body, from [Signatures] on.
	Signature string `json:"signature,omitempty"`
	// Doc is the documentation comment, from [Docs] on.
	Doc string `json:"doc,omitempty"`
	// Visibility is the visibility of a declaration that is not [sema.Exported].
	Visibility sema.Visibility `json:"visibility,omitempty"`
	// Modifiers are the keywords on the declaration, from [Signatures] on.
	Modifiers []string `json:"modifiers,omitempty"`
	// Annotations are the names of the annotations of the declaration, from [Signatures] on.
	Annotations []string `json:"annotations,omitempty"`
	// Snippet is the source text of the declaration, at [Source].
	Snippet string `json:"snippet,omitempty"`
	// Span is the span that the declaration covers, at [Source].
	Span *Extent `json:"span,omitempty"`
	// Members are the declarations that this one contains, such as the fields of a struct.
	Members Members `json:"members,omitempty"`
	// qualified is the qualified name of the declaration, as Instant.Time for the method Time of
	// Instant, which [Narrow] matches beside the name. It is not part of the answer.
	qualified string
}

// Declared returns the declarations of items at the level d, each nested under the smallest
// declaration whose span contains it, as [sema.Containers] computes. It keeps the bindings that
// include selects, as [engine.Bindings.Keeps] decides from the kind and from [sema.Locals]. A
// declaration whose container it leaves out goes under the nearest container that it keeps.
// The order is the order of items.
//
// A declaration with source text has no members, because its source text contains them. At
// [Summaries] a member without a summary, a type and members of its own is left out, because
// its line would state only its name. Declared returns an empty list, not nil, for no
// declarations.
func Declared(items []sema.Symbol, d Detail, include engine.Bindings) []Declaration {
	return declared(items, d, include, false)
}

// Resolved returns the declarations that a name denotes as [Declared] does, and keeps each
// declaration that no other item contains whatever include selects: the caller asked what the
// name denotes, and a parameter or an import is an answer. include selects their members.
func Resolved(items []sema.Symbol, d Detail, include engine.Bindings) []Declaration {
	return declared(items, d, include, true)
}

// declared returns the declarations of [Declared], and keeps every declaration that no other
// item contains when rooted is true.
func declared(items []sema.Symbol, d Detail, include engine.Bindings, rooted bool) []Declaration {
	containers := sema.Containers(items)
	locals := sema.Locals(items, containers)
	keep := make([]bool, len(items))
	for i, s := range items {
		keep[i] = rooted && containers[i] < 0 || include.Keeps(s.Kind, locals[i])
	}

	children := make([][]int, len(items))
	var roots []int
	for i := range items {
		if !keep[i] {
			continue
		}
		up := containers[i]
		for up >= 0 && !keep[up] {
			up = containers[up]
		}
		if up < 0 {
			roots = append(roots, i)
			continue
		}
		children[up] = append(children[up], i)
	}

	var build func(i int) Declaration
	build = func(i int) Declaration {
		out := project(items[i], d)
		if out.Snippet != "" {
			return out
		}
		for _, child := range children[i] {
			member := build(child)
			if d == Summaries && member.Summary == "" && member.Signature == "" && len(member.Members) == 0 {
				continue
			}
			out.Members = append(out.Members, member)
		}
		return out
	}
	out := []Declaration{}
	for _, i := range roots {
		out = append(out, build(i))
	}
	return out
}

// project returns the declaration of s at the level d. It sets the visibility at every level,
// because [Narrow] reads it.
func project(s sema.Symbol, d Detail) Declaration {
	out := Declaration{
		Name:      s.Name,
		Kind:      s.Kind,
		Line:      s.Span.Start.Line + 1,
		Path:      string(s.Span.Path),
		qualified: s.ID.Name(),
	}
	if s.Visibility != sema.Exported {
		out.Visibility = s.Visibility
	}
	switch d {
	case Names:
		out.Signature = fieldType(s)
		return out
	case Summaries:
		out.Signature, out.Summary = fieldType(s), summary(s.Doc)
		return out
	case Signatures, Docs, Source, DetailUnset:
	}

	out.Signature = s.Signature
	out.Modifiers = s.Modifiers
	for _, a := range s.Annotations {
		out.Annotations = append(out.Annotations, a.Name)
	}
	if d == Signatures {
		return out
	}

	out.Doc = s.Doc
	if d == Docs {
		return out
	}

	out.Snippet = s.Snippet
	out.Span = &Extent{Start: placeOf(s.Span.Start), End: placeOf(s.Span.End)}
	return out
}

// fieldType returns the signature of a field or a property, which is its name and its type,
// and the empty string for any other kind. [Names] and [Summaries] state the type of a field
// with it, because a field without its type tells a caller only that it exists.
func fieldType(s sema.Symbol) string {
	if s.Kind == sema.KindField || s.Kind == sema.KindProperty {
		return s.Signature
	}
	return ""
}

// summaryLimit is the most bytes of a summary. [summary] cuts a longer first sentence.
const summaryLimit = 160

// summary returns the first sentence of doc on one line: the text of the first paragraph up
// to its first full stop before a space or at its end, with its white space collapsed to
// single spaces. The paragraph ends at a blank line and before a line that starts a list
// item. A sentence longer than [summaryLimit] bytes is cut at the last space before the
// limit, or at the limit without one, and ends with an ellipsis. No documentation has no
// summary.
func summary(doc string) string {
	var paragraph []string
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && len(paragraph) > 0 || listed(trimmed) {
			break
		}
		paragraph = append(paragraph, trimmed)
	}
	text := strings.Join(strings.Fields(strings.Join(paragraph, " ")), " ")
	if at := strings.Index(text+" ", ". "); at >= 0 {
		text = text[:at+1]
	}
	if len(text) <= summaryLimit {
		return text
	}
	cut := strings.LastIndexByte(text[:summaryLimit], ' ')
	if cut <= 0 {
		cut = summaryLimit
		for !utf8.RuneStart(text[cut]) {
			cut--
		}
	}
	return text[:cut] + "…"
}

// unitSummary returns the [summary] of the first documentation of a package or a module in
// items, which is what the unit of the declarations is for, or the empty string for none.
func unitSummary(items []sema.Symbol) string {
	for _, s := range items {
		if (s.Kind == sema.KindPackage || s.Kind == sema.KindModule) && s.Doc != "" {
			return summary(s.Doc)
		}
	}
	return ""
}

// bullets are the markers that start an item of a list without numbers in documentation, as
// godoc, Markdown and reStructuredText write it.
var bullets = []string{"- ", "* ", "+ "}

// listed reports whether a trimmed line of documentation starts a list item: one of
// [bullets], or a number and a full stop and a space.
func listed(line string) bool {
	for _, marker := range bullets {
		if strings.HasPrefix(line, marker) {
			return true
		}
	}
	digits := strings.TrimLeft(line, "0123456789")
	return len(digits) < len(line) && strings.HasPrefix(digits, ". ")
}

// Extent is the half-open range of source text that a declaration covers, in the file of its
// item: it includes Start and excludes End.
type Extent struct {
	Start Place `json:"start"`
	End   Place `json:"end"`
}

// Place is one position of an [Extent]. Line and Column count from one, as every line and column
// of an answer does, and Offset counts bytes from zero, as a caller slices the file.
type Place struct {
	Line int `json:"line"`
	// Column is the column in bytes.
	Column int `json:"column"`
	Offset int `json:"offset"`
}

// placeOf returns p as a [Place].
func placeOf(p source.Position) Place {
	return Place{Line: p.Line + 1, Column: p.Column + 1, Offset: p.Offset}
}
