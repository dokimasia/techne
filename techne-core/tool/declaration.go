// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"slices"
	"sort"
	"strings"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Include names what an answer holds beyond the declarations a file
// offers to the rest of a program.
//
// The default is those declarations alone. A parameter belongs to a
// signature and the signature is on the declaration that owns it; a
// binding local to a body is not something the file offers. Reporting
// either as a peer of the function it sits in is what makes an outline
// cost more than the file.
type Include string

const (
	// IncludeImport adds what the file brings into scope.
	IncludeImport Include = "import"
	// IncludeParameter adds the bindings in each signature.
	IncludeParameter Include = "parameter"
	// IncludeLocal adds the declarations inside a callable's body.
	IncludeLocal Include = "local"
	// IncludeAll adds every name the file binds.
	IncludeAll Include = "all"
)

// asks reports whether a caller named one of these.
func asks(named []string, what Include) bool {
	for _, one := range named {
		if Include(one) == what || Include(one) == IncludeAll {
			return true
		}
	}
	return false
}

// Narrow limits an answer to the declarations a caller named.
//
// It is what makes the levels that carry documentation and source text
// worth calling. Over a whole file those levels cost more than reading
// the file, because the source text of every declaration is the file;
// over a handful of named declarations they cost a fraction of it.
type Narrow struct {
	// Names limits the answer to these declarations. A qualified name
	// matches the declaration it names, whatever holds it.
	Names []string
	// Kind limits it to one kind. The zero value matches any.
	Kind sema.Kind
	// Prefix limits it to names starting with this.
	Prefix string
	// Private includes declarations not visible outside their unit.
	Private bool
}

// wanted reports whether one declaration is what the caller asked for.
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
	if len(n.Names) == 0 {
		return true
	}
	return slices.Contains(n.Names, d.Name)
}

// Apply keeps what a caller asked for, and what it holds.
//
// A declaration that matches is kept whole, because asking for a struct
// means asking for its fields. One that does not match survives only to
// carry a match below it, and then holds nothing else: asking for every
// method is not asking for the fields beside them.
func (n Narrow) Apply(items []Declaration) []Declaration {
	out := []Declaration{}
	for _, item := range items {
		if n.wanted(item) {
			out = append(out, item)
			continue
		}
		if held := n.Apply(item.Members); len(held) > 0 {
			item.Members = held
			out = append(out, item)
		}
	}
	return out
}

// Members are the declarations one declaration holds.
//
// It is a type of its own so a schema can describe it by pointing at the
// description of a declaration rather than by containing one, which does
// not terminate.
type Members []Declaration

// Declaration is one item of an answer, in the form a caller reads.
//
// It is not the engine's record. An engine returns what it found, in a
// shape sized for an index; this is what the question asked for, with
// the facts an answer states once left in [Scope] and the fields a
// caller does not act on left out.
type Declaration struct {
	Name string    `json:"name"`
	Kind sema.Kind `json:"kind"`
	// Line is where the declaration begins, counted from one, because
	// that is how an editor and a reader count.
	Line int `json:"line"`
	// Path is carried only where an answer spans more than one file.
	// Otherwise the scope names it.
	Path string `json:"path,omitempty"`
	// Signature is the declaration without its body.
	Signature string `json:"signature,omitempty"`
	// Doc is the documentation comment, in whichever form the language
	// writes one.
	Doc string `json:"doc,omitempty"`
	// Visibility is carried when it is not [sema.Exported], because a
	// request returns exported declarations unless it asked otherwise
	// and repeating the common answer costs a word per item.
	Visibility sema.Visibility `json:"visibility,omitempty"`
	Modifiers  []string        `json:"modifiers,omitempty"`
	// Annotations are the names of what is written onto the declaration.
	// The whole text of one is what a rewriting tool needs, and reaches
	// a caller at [Source].
	Annotations []string `json:"annotations,omitempty"`
	// Snippet is the declaration's own source, at [Source] alone.
	Snippet string `json:"snippet,omitempty"`
	// Span is the bytes the declaration covers, at [Source] alone,
	// because slicing them is what the write path does and reading does
	// not.
	Span *source.Span `json:"span,omitempty"`
	// Members are the declarations this one holds: a struct's fields, a
	// class's methods. Nesting is what tells a package-level binding
	// from one local to a body, which no kind distinguishes.
	Members Members `json:"members,omitempty"`
}

// Declared turns what an engine found into what a caller reads, nesting
// each declaration inside the one that contains it.
//
// Containment is decided by the bytes a declaration covers, so it holds
// for every language without a query having to say what encloses what.
// The order is the order the engine returned, which for a parser is the
// order of the source.
func Declared(items []sema.Symbol, d Detail, include []string) []Declaration {
	held := make([]int, len(items))
	for i := range held {
		held[i] = -1
	}

	// Smallest first, so the first container found is the nearest one.
	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return width(items[order[a]]) < width(items[order[b]])
	})
	for _, i := range order {
		for _, j := range order {
			if i == j || width(items[j]) <= width(items[i]) {
				continue
			}
			if contains(items[j], items[i]) {
				held[i] = j
				break
			}
		}
	}

	keep := kept(items, held, include)

	// A declaration whose container was dropped rises to the nearest one
	// that was kept, so asking for locals without asking for parameters
	// still puts each local under its function.
	children := make([][]int, len(items))
	var roots []int
	for i := range items {
		if !keep[i] {
			continue
		}
		up := held[i]
		for up >= 0 && !keep[up] {
			up = held[up]
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
		// A declaration's own source text already holds everything
		// inside it. Carrying the members beside it would send a
		// struct's fields twice, once as text and once as items.
		if out.Snippet != "" {
			return out
		}
		for _, child := range children[i] {
			out.Members = append(out.Members, build(child))
		}
		return out
	}

	// An answer that found nothing says so with an empty list rather
	// than a null, because a caller reads the two the same way only if
	// it remembers to.
	out := []Declaration{}
	for _, i := range roots {
		out = append(out, build(i))
	}
	return out
}

// kept decides which declarations an answer holds.
//
// A kind that binds a name never leaving its scope is dropped unless the
// caller asked for it, and so is anything written inside a callable: the
// question an outline answers is what a file offers, and neither is part
// of that.
func kept(items []sema.Symbol, held []int, include []string) []bool {
	keep := make([]bool, len(items))
	for i, s := range items {
		switch {
		case s.Kind == sema.KindImport:
			keep[i] = asks(include, IncludeImport)
		case s.Kind == sema.KindParameter || s.Kind == sema.KindTypeParameter:
			keep[i] = asks(include, IncludeParameter)
		case s.Kind == sema.KindLabel:
			keep[i] = asks(include, IncludeAll)
		case insideValue(items, held, i):
			keep[i] = asks(include, IncludeLocal)
		default:
			keep[i] = true
		}
	}
	return keep
}

// insideValue reports whether a declaration is written inside a
// callable's body or inside a value.
//
// Nothing in the vocabulary separates a package-level binding from one
// local to a function: both are variables. Where it sits is what
// separates them, and the same holds one level out. A key in an object
// literal is a binding the language makes, and a caller asking what a
// file offers is not asking for the fields of a value passed to a
// constructor.
func insideValue(items []sema.Symbol, held []int, of int) bool {
	for at, steps := held[of], 0; at >= 0 && steps <= len(items); at, steps = held[at], steps+1 {
		switch items[at].Kind {
		case sema.KindFunction, sema.KindMethod, sema.KindConstructor, sema.KindProperty,
			sema.KindField, sema.KindVariable, sema.KindConstant:
			return true
		}
	}
	return false
}

// project returns one declaration carrying what the level does.
func project(s sema.Symbol, d Detail) Declaration {
	out := Declaration{
		Name: s.Name,
		Kind: s.Kind,
		// A span counts from zero and a reader counts from one.
		Line: s.Span.Start.Line + 1,
		Path: string(s.Span.Path),
	}
	if d == Names {
		return out
	}

	out.Signature = s.Signature
	out.Modifiers = s.Modifiers
	if s.Visibility != sema.Exported {
		out.Visibility = s.Visibility
	}
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

func width(s sema.Symbol) int {
	return s.Span.End.Offset - s.Span.Start.Offset
}

func contains(outer, inner sema.Symbol) bool {
	return outer.Span.Path == inner.Span.Path &&
		outer.Span.Start.Offset <= inner.Span.Start.Offset &&
		outer.Span.End.Offset >= inner.Span.End.Offset
}
