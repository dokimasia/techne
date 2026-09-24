// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"fmt"
	"sort"
	"strings"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Detail is the level of detail of each declaration of an answer. Each level contains the
// levels before it in [Levels]. The zero value is [DetailUnset], which selects
// [DefaultDetail].
type Detail string

const (
	// DetailUnset means the caller named no level.
	DetailUnset Detail = ""
	// Names is the name, the kind and the line of a declaration.
	Names Detail = "names"
	// Signatures adds the declaration without its body, its modifiers and the names of its
	// annotations.
	Signatures Detail = "signatures"
	// Docs adds the documentation comment.
	Docs Detail = "docs"
	// Source adds the source text of the declaration and the span it covers.
	Source Detail = "source"
)

// Levels returns every level, the smallest first.
func Levels() []Detail { return []Detail{Names, Signatures, Docs, Source} }

// String returns the word of d.
func (d Detail) String() string { return string(d) }

// DefaultDetail returns the level for a scope whose caller named none: [Signatures] for a
// scope that names one file, and [Names] for a directory.
func DefaultDetail(scope source.Path) Detail {
	if !names(scope) {
		return Names
	}
	return Signatures
}

// DefaultMaxTokens is the budget of a caller that names none.
const DefaultMaxTokens = 6000

// bytesPerToken is the number of bytes of a rendered answer that the budget counts as one
// token. Code and identifiers take fewer bytes per token than English, so the estimate is
// high.
const bytesPerToken = 3

// Budget is how much of an answer a caller takes. [Fit] applies it after the level of
// [Detail] selected the fields of each declaration.
type Budget struct {
	// MaxTokens is the ceiling, in tokens of three bytes of the rendered answer. Zero means
	// [DefaultMaxTokens].
	MaxTokens int
}

// Fit removes detail from a in these steps, and returns a after the first step at which it
// fits b:
//
//  1. the documentation of every declaration
//  2. the source text of every declaration
//  3. the labels, the imports, the type parameters and the parameters, in that order
//  4. the members at the deepest level, one level at a time
//  5. the declarations that are not visible outside their unit
//  6. the declarations at the end of the answer, down to one
//
// Fit then puts back the source text, and then the documentation, of the declarations that it
// returns, each whole and in the order of the answer, while the answer fits. A declaration
// whose source text does not fit gets no documentation back.
//
// A thinned answer has one [trust.CaveatTruncated] caveat. Its note counts the declarations
// that matched and that the answer returns, the declarations dropped by kind, and the
// declarations returned without their documentation or their source text. Fit returns an
// answer with that caveat as it is, and does not change the rest of the provenance.
//
// The estimate is the length of [Answer.Render] without the note of the caveat. Fit reads
// the cost of each declaration once, and every estimate adds up the stored costs.
func Fit(a Answer, b Budget) Answer {
	ceiling := b.MaxTokens
	if ceiling <= 0 {
		ceiling = DefaultMaxTokens
	}
	matched := count(a.Items)
	if matched == 0 || cut(a) {
		return a
	}
	f := fitting(a, ceiling)
	if f.fits(f.cost()) {
		return a
	}
	f.thin()
	f.restore()
	return f.answer(matched)
}

// cut reports whether an answer has a truncation caveat.
func cut(a Answer) bool {
	for _, c := range a.Provenance.Caveats {
		if c.Code == string(trust.CaveatTruncated) {
			return true
		}
	}
	return false
}

// fit is an answer that [Fit] thins: each declaration once, in the order of the answer with
// the members of each after it, and the bytes that the render of each takes.
type fit struct {
	a     Answer
	nodes []node
	// limit is the largest render in bytes that fits the budget.
	limit int
	// fixed is the bytes of the render outside the declarations, except for the count of the
	// heading, which [fit.cost] adds.
	fixed int
}

// node is one declaration of a [fit], without its members.
type node struct {
	d     Declaration
	depth int
	// end is the index after the last member of the declaration, at any depth.
	end int
	// head is the bytes of the first line with the source text, thin without it, rest the
	// bytes of the other lines of the source text, and doc the bytes of the documentation.
	head, thin, rest, doc int
	// dropped removes the declaration and its members. undocumented and unsnipped remove its
	// documentation and its source text.
	dropped, undocumented, unsnipped bool
}

// fitting returns the fit of a under a ceiling in tokens.
func fitting(a Answer, ceiling int) *fit {
	f := &fit{a: a, limit: (ceiling+1)*bytesPerToken - 1}
	f.fixed = len("\n\n") + len("\n") + len(evidence(a.Provenance))
	var flatten func(items []Declaration, depth int)
	flatten = func(items []Declaration, depth int) {
		for _, item := range items {
			at := len(f.nodes)
			own := item
			own.Members = nil
			f.nodes = append(f.nodes, costed(own, depth, a.Scope.Path == ""))
			flatten(item.Members, depth+1)
			f.nodes[at].end = len(f.nodes)
		}
	}
	flatten(a.Items, 0)
	return f
}

// costed returns the node of d at depth, with the bytes of each part of its render.
func costed(d Declaration, depth int, withPath bool) node {
	out := node{d: d, depth: depth}
	indent := strings.Repeat("  ", depth)
	head, rest := parts(d)
	out.head = len(headLine(indent, d, head, withPath))
	for _, line := range rest {
		out.rest += len(bodyLine(indent, line))
	}
	thin := d
	thin.Snippet = ""
	thinHead, _ := parts(thin)
	out.thin = len(headLine(indent, d, thinHead, withPath))
	if d.Doc != "" {
		for line := range strings.SplitSeq(d.Doc, "\n") {
			out.doc += len(bodyLine(indent, line))
		}
	}
	return out
}

// own returns the bytes of the render of n without its members.
func (n node) own() int {
	out := n.thin
	if n.d.Snippet != "" && !n.unsnipped {
		out = n.head + n.rest
	}
	if n.d.Doc != "" && !n.undocumented {
		out += n.doc
	}
	return out
}

// cost returns the bytes of the render of the declarations that f keeps.
func (f *fit) cost() int {
	total, held := f.fixed, 0
	for i := 0; i < len(f.nodes); {
		if f.nodes[i].dropped {
			i = f.nodes[i].end
			continue
		}
		total += f.nodes[i].own()
		held++
		i++
	}
	return total + len(headed(f.a.Scope, held))
}

// fits reports whether a render of total bytes fits the budget.
func (f *fit) fits(total int) bool { return total <= f.limit }

// thin removes detail by the steps of [Fit] until the answer fits.
func (f *fit) thin() {
	steps := []func(){
		func() { f.each(func(n *node) { n.undocumented = true }) },
		func() { f.each(func(n *node) { n.unsnipped = true }) },
	}
	for _, kind := range []sema.Kind{
		sema.KindLabel, sema.KindImport, sema.KindTypeParameter, sema.KindParameter,
	} {
		steps = append(steps, f.dropping(func(n *node) bool { return n.d.Kind == kind }))
	}
	for depth := f.deepest(); depth > 0; depth-- {
		steps = append(steps, f.dropping(func(n *node) bool { return n.depth == depth }))
	}
	steps = append(steps, f.dropping(func(n *node) bool { return n.d.Visibility == sema.Unexported }))
	for _, step := range steps {
		step()
		if f.fits(f.cost()) {
			f.keepOne()
			return
		}
	}
	f.keepOne()
	f.shorten()
}

// keepOne keeps the first declaration at the top level when f drops every declaration there,
// because an answer that matched something returns at least one declaration.
func (f *fit) keepOne() {
	for i := 0; i < len(f.nodes); i = f.nodes[i].end {
		if !f.nodes[i].dropped {
			return
		}
	}
	if len(f.nodes) > 0 {
		f.nodes[0].dropped = false
	}
}

// each applies change to every node.
func (f *fit) each(change func(n *node)) {
	for i := range f.nodes {
		change(&f.nodes[i])
	}
}

// dropping returns a step that drops every node that rule selects, with its members.
func (f *fit) dropping(rule func(n *node) bool) func() {
	return func() {
		f.each(func(n *node) {
			if rule(n) {
				n.dropped = true
			}
		})
	}
}

// deepest returns the largest depth of a node.
func (f *fit) deepest() int {
	most := 0
	for _, n := range f.nodes {
		most = max(most, n.depth)
	}
	return most
}

// shorten keeps the longest run of declarations at the top level, from the first, whose
// render fits, and at least the first.
func (f *fit) shorten() {
	var tops []int
	for i, n := range f.nodes {
		if n.depth == 0 && !n.dropped {
			tops = append(tops, i)
		}
	}
	keep := func(k int) {
		for j, at := range tops {
			f.nodes[at].dropped = j >= k
		}
	}
	keep(longest(len(tops), func(k int) bool {
		keep(k)
		return f.fits(f.cost())
	}))
}

// longest returns the largest k from 1 to n for which fits reports true, or 1 when it reports
// true for none. fits must report true for every k below one for which it reports true, as a
// shorter run of an answer never costs more. The binary search calls fits as many times as n
// has bits.
func longest(n int, fits func(k int) bool) int {
	low, high := 1, n
	for low < high {
		half := (low + high + 1) / 2
		if fits(half) {
			low = half
		} else {
			high = half - 1
		}
	}
	return low
}

// restore puts back the source text of each node that f keeps, and then its documentation,
// in order and while the answer fits. A node whose source text is left out gets no
// documentation back.
func (f *fit) restore() {
	total := f.cost()
	kept := f.kept()
	for _, i := range kept {
		n := &f.nodes[i]
		if n.d.Snippet == "" || !n.unsnipped {
			continue
		}
		if grown := total - n.own(); f.fits(grown + n.head + n.rest + n.carried()) {
			n.unsnipped = false
			total = grown + n.own()
		}
	}
	for _, i := range kept {
		n := &f.nodes[i]
		if n.d.Doc == "" || !n.undocumented || n.d.Snippet != "" && n.unsnipped {
			continue
		}
		if f.fits(total + n.doc) {
			n.undocumented = false
			total += n.doc
		}
	}
}

// carried returns the bytes of the documentation of n that the answer keeps.
func (n node) carried() int {
	if n.d.Doc == "" || n.undocumented {
		return 0
	}
	return n.doc
}

// kept returns the indexes of the nodes that f keeps, in order.
func (f *fit) kept() []int {
	var out []int
	for i := 0; i < len(f.nodes); {
		if f.nodes[i].dropped {
			i = f.nodes[i].end
			continue
		}
		out = append(out, i)
		i++
	}
	return out
}

// answer returns the answer that f keeps, with the truncation caveat when f removed anything.
func (f *fit) answer(matched int) Answer {
	dropped := map[string]int{}
	undocumented, unsnipped := 0, 0
	var build func(from, to int) []Declaration
	build = func(from, to int) []Declaration {
		out := []Declaration{}
		for i := from; i < to; {
			n := f.nodes[i]
			if n.dropped {
				for _, gone := range f.nodes[i:n.end] {
					dropped[gone.d.Kind.String()]++
				}
				i = n.end
				continue
			}
			d := n.d
			if d.Doc != "" && n.undocumented {
				d.Doc = ""
				undocumented++
			}
			if d.Snippet != "" && n.unsnipped {
				d.Snippet = ""
				unsnipped++
			}
			if members := build(i+1, n.end); len(members) > 0 {
				d.Members = members
			}
			out = append(out, d)
			i = n.end
		}
		return out
	}

	out := f.a
	out.Items = build(0, len(f.nodes))
	held := count(out.Items)
	if held == matched && undocumented == 0 && unsnipped == 0 {
		return out
	}
	note := fmt.Sprintf("%d matched, %d returned", matched, held)
	if by := sorted(dropped); by != "" {
		note += ". Dropped " + by
	}
	if undocumented > 0 {
		note += ". " + plural(undocumented, "declaration", "declarations") + " without documentation"
	}
	if unsnipped > 0 {
		note += ". " + plural(unsnipped, "declaration", "declarations") + " without source text"
	}
	out.Provenance.Caveats = append(append([]Caveat(nil), out.Provenance.Caveats...), Caveat{
		Code: string(trust.CaveatTruncated),
		Note: note,
	})
	return out
}

// sorted returns the counts of dropped by kind, the largest first and then by kind.
func sorted(dropped map[string]int) string {
	kinds := make([]string, 0, len(dropped))
	for kind := range dropped {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(a, b int) bool {
		if dropped[kinds[a]] != dropped[kinds[b]] {
			return dropped[kinds[a]] > dropped[kinds[b]]
		}
		return kinds[a] < kinds[b]
	})
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", dropped[kind], kind))
	}
	return strings.Join(parts, ", ")
}
