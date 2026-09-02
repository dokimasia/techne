// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Scope is what an answer is about, as against how it was reached.
//
// A request routes to one language and usually to one unit, so both are
// stated here rather than on every item. An item names its own path only
// where the answer spans more than one file.
type Scope struct {
	Language string `json:"language,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Path     string `json:"path,omitempty"`
}

// Answer is the shape every read tool returns.
//
// There is no status. An answer that ran states its evidence in
// [Provenance] and what it could not cover in the caveats, and an answer
// that could not run carries [Failure] instead. A status field would
// restate one of the two.
type Answer struct {
	Scope      Scope         `json:"scope"`
	Items      []Declaration `json:"items"`
	Provenance Provenance    `json:"provenance"`
	// Error is present only when nothing could be served, and is what
	// makes the result a failure a model can act on.
	Error *Failure `json:"error,omitempty"`
}

// Failure says why a call could not be served.
type Failure struct {
	// Code is unsupported when nothing serves this language and role,
	// and refused when the system declined or the target does not
	// exist. An agent routes around the first and corrects the second.
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// Failed reports whether a caller should read this answer as a failure.
func (a Answer) Failed() bool { return a.Error != nil }

// Provenance is what stands behind an answer, in the form a caller
// reads.
type Provenance struct {
	Engine       string `json:"engine,omitempty"`
	Fidelity     string `json:"fidelity"`
	Completeness string `json:"completeness"`
	// SupportsNegativeClaim saves a caller knowing that this means
	// resolved binding together with total coverage.
	SupportsNegativeClaim bool     `json:"supportsNegativeClaim"`
	Caveats               []Caveat `json:"caveats,omitempty"`
}

// Caveat is a limit on an answer that its tier does not express.
type Caveat struct {
	Code  string        `json:"code"`
	Note  string        `json:"note,omitempty"`
	Paths []source.Path `json:"paths,omitempty"`
}

// published turns what an engine returned into what a caller reads,
// where every tier is a word rather than a number.
//
// A call nothing could serve becomes a [Failure] rather than an empty
// answer with a status, because the two need different shapes: one says
// what was found and how it was bound, the other says why there is
// nothing to say.
func published(a engine.Answer[sema.Symbol], scope Scope, d Detail, include []string) Answer {
	caveats := make([]Caveat, 0, len(a.Provenance.Caveats))
	for _, c := range a.Provenance.Caveats {
		caveats = append(caveats, Caveat{Code: string(c.Code), Note: c.Note, Paths: c.Paths})
	}

	out := Answer{
		Scope: scope,
		Items: Declared(a.Items, d, include),
		Provenance: Provenance{
			Engine:                a.Provenance.Engine,
			Fidelity:              a.Provenance.Fidelity.String(),
			Completeness:          a.Provenance.Completeness.String(),
			SupportsNegativeClaim: a.Provenance.SupportsNegativeClaim(),
			Caveats:               caveats,
		},
	}

	switch a.Status {
	case trust.Unsupported:
		out.Error = &Failure{Code: trust.Unsupported.String(), Reason: reasonFrom(caveats)}
	case trust.Refused:
		out.Error = &Failure{Code: trust.Refused.String(), Reason: reasonFrom(caveats)}
	case trust.Unset, trust.OK, trust.Degraded, trust.Partial:
	}
	return out
}

// reasonFrom reads why a call could not be served out of the caveat the
// service put it in.
func reasonFrom(caveats []Caveat) string {
	for _, c := range caveats {
		if c.Note != "" {
			return c.Note
		}
	}
	return "nothing serves this request"
}

// Render writes the answer for a reader rather than a parser.
//
// A tool result carries an unstructured block and a structured one, and
// they have different readers: a model reads the first and a program
// reads the second. Serialising the same JSON into both spends the
// larger of the two costs twice and hands the model the form it reads
// worst.
func (a Answer) Render() string {
	var b strings.Builder

	if a.Error != nil {
		fmt.Fprintf(&b, "%s: %s\n", a.Error.Code, a.Error.Reason)
		return b.String()
	}

	b.WriteString(a.heading())
	b.WriteString("\n\n")
	if len(a.Items) == 0 {
		b.WriteString("nothing declared\n\n")
	}
	for _, item := range a.Items {
		item.render(&b, 0, a.Scope.Path == "")
	}
	b.WriteString("\n")
	b.WriteString(a.evidence())
	return b.String()
}

// heading names what the answer is about and how much of it there is.
func (a Answer) heading() string {
	about := a.Scope.Path
	if about == "" {
		about = a.Scope.Unit
	}
	unit := a.Scope.Unit
	if unit == "." {
		unit = ""
	}
	held := strings.TrimSpace(a.Scope.Language + " " + unit)
	return fmt.Sprintf("%s — %s, %s", about, held, plural(count(a.Items), "declaration", "declarations"))
}

// evidence is the footer: what bound the answer, how much it covered,
// and every limit on it.
func (a Answer) evidence() string {
	var b strings.Builder
	b.WriteString(a.Provenance.Fidelity + ", " + a.Provenance.Completeness + " coverage")
	if a.Provenance.SupportsNegativeClaim {
		b.WriteString(". an empty answer here means there are none")
	}
	for _, c := range a.Provenance.Caveats {
		b.WriteString(". " + c.Note)
	}
	b.WriteString(".\n")
	return b.String()
}

// plural counts a thing without reading as a bug at one of it.
//
// Both forms are given rather than a suffix added, because English does
// not make the plural of match by adding one.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// body writes the items and the evidence, which every answer shares
// whatever it heads them with.
func (a Answer) body(b *strings.Builder) {
	if len(a.Items) == 0 {
		b.WriteString("nothing found\n")
	}
	for _, item := range a.Items {
		item.render(b, 0, a.Scope.Path == "")
	}
	b.WriteString("\n")
	b.WriteString(a.evidence())
}

// render writes one declaration and everything it holds.
//
// A level that was asked for is written out. Truncating the
// documentation to its first line or leaving the source text out would
// make the half a model reads carry less than the half it does not,
// which is the failure the two renderings exist to avoid.
func (d Declaration) render(b *strings.Builder, depth int, withPath bool) {
	indent := strings.Repeat("  ", depth)

	// The source text opens with the declaration's own signature, so
	// writing the signature above it would say it twice.
	head, rest := d.Signature, []string(nil)
	if d.Snippet != "" {
		lines := strings.Split(d.Snippet, "\n")
		head, rest = lines[0], lines[1:]
	}
	if head == "" {
		head = d.Kind.String() + " " + d.Name
	}

	if withPath && d.Path != "" {
		fmt.Fprintf(b, "%s%s:%d  %s\n", indent, d.Path, d.Line, head)
	} else {
		fmt.Fprintf(b, "%s%5d  %s\n", indent, d.Line, head)
	}
	for _, line := range rest {
		fmt.Fprintf(b, "%s%7s%s\n", indent, "", line)
	}
	if d.Doc != "" {
		for line := range strings.SplitSeq(d.Doc, "\n") {
			fmt.Fprintf(b, "%s%7s%s\n", indent, "", line)
		}
	}

	for _, member := range d.Members {
		member.render(b, depth+1, withPath)
	}
}

// count reports how many declarations an answer holds, members included.
func count(items []Declaration) int {
	total := len(items)
	for _, item := range items {
		total += count(item.Members)
	}
	return total
}
