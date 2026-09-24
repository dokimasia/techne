// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import (
	"cmp"
	"slices"

	"go.dokimi.dev/techne/core/source"
)

// Annotation is metadata attached to a declaration: a Java annotation, a
// Python or TypeScript decorator, a Rust or C# attribute, or a Go struct tag.
// An annotation declares nothing, so it is not a Symbol.
type Annotation struct {
	// Name is the annotation without punctuation or arguments: "Injectable"
	// for @Injectable({scope: 1}), "derive" for #[derive(Debug)], and "json"
	// for the struct tag `json:"id"`.
	Name string `json:"name"`
	// Text is the annotation exactly as written.
	Text string `json:"text"`
	// Span is the source range of the annotation itself.
	Span source.Span `json:"span"`
}

// Symbol is one declaration as an engine reports it. The fields after Span
// are optional, and an engine sets each one that it can extract.
type Symbol struct {
	ID       ID              `json:"id"`
	Name     string          `json:"name"`
	Kind     Kind            `json:"kind"`
	Language source.Language `json:"language"`
	Span     source.Span     `json:"span"`
	// Parent is the ID of the enclosing declaration, or empty at the top
	// level of a unit.
	Parent ID `json:"parent,omitempty"`
	// Visibility is always encoded, because VisibilityUnknown is a result.
	Visibility Visibility `json:"visibility"`
	// Modifiers are the keywords on the declaration, in source order, such
	// as static, final, async, pub, or export.
	Modifiers []string `json:"modifiers,omitempty"`
	// Annotations are the annotations attached to the declaration.
	Annotations []Annotation `json:"annotations,omitempty"`
	// Signature is the declaration without its body.
	Signature string `json:"signature,omitempty"`
	// Doc is the documentation comment.
	Doc string `json:"doc,omitempty"`
	// Snippet is the source text of the declaration.
	Snippet string `json:"snippet,omitempty"`
}

// Annotated reports whether s has an annotation named name.
func (s Symbol) Annotated(name string) bool {
	for _, a := range s.Annotations {
		if a.Name == name {
			return true
		}
	}
	return false
}

// Modified reports whether s has the modifier keyword.
func (s Symbol) Modified(keyword string) bool {
	return slices.Contains(s.Modifiers, keyword)
}

// Unit is the grouping a file belongs to: a package in Go, a module in
// Python, a crate module in Rust.
type Unit struct {
	ID       ID              `json:"id"`
	Name     string          `json:"name"`
	Language source.Language `json:"language"`
	Root     source.Path     `json:"root"`
	Files    []source.Path   `json:"files,omitempty"`
}

// Containers returns, for each symbol, the index of the smallest other
// symbol in the same file whose span contains it and is wider, or -1 if
// there is none. Equal spans do not contain each other. Of two containers
// of equal width, Containers returns the one that appears first in symbols.
//
// Spans are expected to nest as a tree, as parsers and language servers
// produce them. If two spans partially overlap, the result for the symbols
// inside both is a container but not necessarily the smallest one.
//
// Containers runs in O(n log n) time.
func Containers(symbols []Symbol) []int {
	out := make([]int, len(symbols))
	order := make([]int, len(symbols))
	for i := range symbols {
		out[i], order[i] = -1, i
	}

	// Sort outer spans before the spans they contain. Equal spans sort in
	// reverse input order, which leaves the first of them on top of the
	// stack.
	slices.SortFunc(order, func(a, b int) int {
		sa, sb := symbols[a].Span, symbols[b].Span
		return cmp.Or(
			cmp.Compare(sa.Path, sb.Path),
			cmp.Compare(sa.Start.Offset, sb.Start.Offset),
			cmp.Compare(sb.End.Offset, sa.End.Offset),
			cmp.Compare(b, a),
		)
	})

	var stack []int // spans that contain the current one, outermost first
	for _, i := range order {
		span := symbols[i].Span
		for len(stack) > 0 && !contains(symbols[stack[len(stack)-1]].Span, span) {
			stack = stack[:len(stack)-1]
		}
		for _, container := range slices.Backward(stack) {
			if outer := symbols[container].Span; contains(outer, span) && width(outer) > width(span) {
				out[i] = container
				break
			}
		}
		stack = append(stack, i)
	}
	return out
}

func contains(outer, inner source.Span) bool {
	return outer.Path == inner.Path &&
		outer.Start.Offset <= inner.Start.Offset &&
		inner.End.Offset <= outer.End.Offset
}

func width(s source.Span) int { return s.End.Offset - s.Start.Offset }
