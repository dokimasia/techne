// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Narrow limits an answer to the declarations that a caller names. Over a whole file the
// levels [Docs] and [Source] cost more than the file, and over a few named declarations they
// cost a fraction of it.
type Narrow struct {
	// Names keeps the declarations of these names.
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
	return len(n.Names) == 0 || slices.Contains(n.Names, d.Name)
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
	// Path is the file of the declaration in an answer about more than one file.
	Path string `json:"path,omitempty"`
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
	Span *source.Span `json:"span,omitempty"`
	// Members are the declarations that this one contains, such as the fields of a struct.
	Members Members `json:"members,omitempty"`
}

// Declared returns the declarations of items at the level d, each nested under the smallest
// declaration whose span contains it, as [sema.Containers] computes. It keeps the bindings that
// include selects, as [engine.Bindings.Keeps] decides from the kind and from [sema.Locals]. A
// declaration whose container it leaves out goes under the nearest container that it keeps.
// The order is the order of items.
//
// A declaration with source text has no members, because its source text contains them.
// Declared returns an empty list, not nil, for no declarations.
func Declared(items []sema.Symbol, d Detail, include engine.Bindings) []Declaration {
	containers := sema.Containers(items)
	locals := sema.Locals(items, containers)
	keep := make([]bool, len(items))
	for i, s := range items {
		keep[i] = include.Keeps(s.Kind, locals[i])
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
			out.Members = append(out.Members, build(child))
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
		Name: s.Name,
		Kind: s.Kind,
		Line: s.Span.Start.Line + 1,
		Path: string(s.Span.Path),
	}
	if s.Visibility != sema.Exported {
		out.Visibility = s.Visibility
	}
	if d == Names {
		return out
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
	span := s.Span
	out.Span = &span
	return out
}
