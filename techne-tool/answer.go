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

// Scope is what an answer is about: the language, the unit and the file of the request. An
// item states its own path only in an answer about more than one file.
type Scope struct {
	Language string `json:"language,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Path     string `json:"path,omitempty"`
}

// Answer is the output of the outline, search and resolve tools: the scope, the declarations,
// the evidence of the engine in Provenance, and an Error when no engine served the request.
// There is no status field.
type Answer struct {
	Scope      Scope         `json:"scope"`
	Items      []Declaration `json:"items"`
	Provenance Provenance    `json:"provenance"`
	// Error is the reason that the request was not served, or nil.
	Error *Failure `json:"error,omitempty"`
}

// Failure states why a request was not served.
type Failure struct {
	// Code is unsupported when no engine serves the language and the role, and refused when
	// the request is malformed, names what does not exist or was declined. A caller routes
	// around the first and corrects the second.
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// Failed reports whether the answer has an Error.
func (a Answer) Failed() bool { return a.Error != nil }

// Provenance is the evidence behind an answer, with each tier as a word.
type Provenance struct {
	Engine       string `json:"engine,omitempty"`
	Fidelity     string `json:"fidelity"`
	Completeness string `json:"completeness"`
	// SupportsNegativeClaim reports whether an empty answer proves that there are none, which
	// needs resolved binding over total coverage.
	SupportsNegativeClaim bool     `json:"supportsNegativeClaim"`
	Caveats               []Caveat `json:"caveats,omitempty"`
}

// Caveat is a limit on an answer that its tier does not state.
type Caveat struct {
	Code  string        `json:"code"`
	Note  string        `json:"note,omitempty"`
	Paths []source.Path `json:"paths,omitempty"`
}

// provenance returns p with each tier as a word. Every tool converts its evidence with this
// function, so a read and a write state one piece of evidence in the same words.
func provenance(p trust.Provenance) Provenance {
	out := Provenance{
		Engine:                p.Engine,
		Fidelity:              p.Fidelity.String(),
		Completeness:          p.Completeness.String(),
		SupportsNegativeClaim: p.SupportsNegativeClaim(),
	}
	for _, c := range p.Caveats {
		out.Caveats = append(out.Caveats, Caveat{Code: string(c.Code), Note: c.Note, Paths: c.Paths})
	}
	return out
}

// published returns the answer of a read tool: the declarations of a at the level d with the
// bindings of include, nested by [Declared], and the evidence of a. An answer that no engine
// served has a [Failure] whose reason is the first note of its caveats.
func published(a engine.Answer[sema.Symbol], scope Scope, d Detail, include engine.Bindings) Answer {
	out := Answer{
		Scope:      scope,
		Items:      Declared(a.Items, d, include),
		Provenance: provenance(a.Provenance),
	}
	switch a.Status {
	case trust.Unsupported:
		out.Error = &Failure{Code: trust.Unsupported.String(), Reason: reasonFrom(out.Provenance.Caveats)}
	case trust.Refused:
		out.Error = &Failure{Code: trust.Refused.String(), Reason: reasonFrom(out.Provenance.Caveats)}
	case trust.Unset, trust.OK, trust.Degraded, trust.Partial:
	}
	return out
}

// failed returns the answer of a request that the tool refuses before an engine reads it.
func failed(scope Scope, f *Failure) Answer {
	return Answer{Scope: scope, Items: []Declaration{}, Error: f}
}

// reasonFrom returns the first note of caveats, where a service states why no engine served a
// request.
func reasonFrom(caveats []Caveat) string {
	for _, c := range caveats {
		if c.Note != "" {
			return c.Note
		}
	}
	return "nothing serves this request"
}

// Render returns the answer as text: a heading, each declaration with its members nested one
// level under it, and the evidence. An answer with an Error renders its code and its reason
// alone.
func (a Answer) Render() string {
	var b strings.Builder
	if a.Error != nil {
		fmt.Fprintf(&b, "%s: %s\n", a.Error.Code, a.Error.Reason)
		return b.String()
	}
	b.WriteString(a.heading())
	b.WriteString("\n\n")
	a.body(&b, "nothing declared")
	return b.String()
}

// heading returns the first line of the render of the answer.
func (a Answer) heading() string { return headed(a.Scope, count(a.Items)) }

// headed returns the first line of the render of an answer about scope with n declarations:
// the file or the unit, the language and the unit, and the count.
func headed(scope Scope, n int) string {
	about := scope.Path
	if about == "" {
		about = scope.Unit
	}
	unit := scope.Unit
	if unit == "." {
		unit = ""
	}
	held := strings.TrimSpace(scope.Language + " " + unit)
	return fmt.Sprintf("%s — %s, %s", about, held, plural(n, "declaration", "declarations"))
}

// body writes the declarations of the answer and then its evidence. An answer without
// declarations writes none in their place.
func (a Answer) body(b *strings.Builder, none string) {
	if len(a.Items) == 0 {
		b.WriteString(none + "\n")
	}
	for _, item := range a.Items {
		item.render(b, 0, a.Scope.Path == "")
	}
	b.WriteString("\n")
	b.WriteString(evidence(a.Provenance))
}

// evidence returns the last lines of every render: the fidelity, the completeness, whether an
// empty answer proves absence, and the note of each caveat. Every tool renders its evidence
// with this function.
func evidence(p Provenance) string {
	var b strings.Builder
	b.WriteString(p.Fidelity + ", " + p.Completeness + " coverage")
	if p.SupportsNegativeClaim {
		b.WriteString(". an empty answer here means there are none")
	}
	for _, c := range p.Caveats {
		b.WriteString(". " + c.Note)
	}
	b.WriteString(".\n")
	return b.String()
}

// plural returns n with the singular form one for a count of 1 and the plural form many for
// any other count.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// render writes d and then each of its members one level deeper. It writes all the
// documentation and all the source text of d.
func (d Declaration) render(b *strings.Builder, depth int, withPath bool) {
	indent := strings.Repeat("  ", depth)
	head, rest := parts(d)
	b.WriteString(headLine(indent, d, head, withPath))
	for _, line := range rest {
		b.WriteString(bodyLine(indent, line))
	}
	if d.Doc != "" {
		for line := range strings.SplitSeq(d.Doc, "\n") {
			b.WriteString(bodyLine(indent, line))
		}
	}
	for _, member := range d.Members {
		member.render(b, depth+1, withPath)
	}
}

// parts returns the first line that the render of d writes and the lines of source text
// under it. Source text starts with the signature, so a declaration with source text writes
// the source text in place of the signature. A declaration without either writes its kind and
// its name.
func parts(d Declaration) (string, []string) {
	head, rest := d.Signature, []string(nil)
	if d.Snippet != "" {
		lines := strings.Split(d.Snippet, "\n")
		head, rest = lines[0], lines[1:]
	}
	if head == "" {
		head = d.Kind.String() + " " + d.Name
	}
	return head, rest
}

// headLine returns the first line of the render of d: its path and its line in an answer about
// more than one file, its line otherwise, and then head.
func headLine(indent string, d Declaration, head string, withPath bool) string {
	if withPath && d.Path != "" {
		return fmt.Sprintf("%s%s:%d  %s\n", indent, d.Path, d.Line, head)
	}
	return fmt.Sprintf("%s%5d  %s\n", indent, d.Line, head)
}

// bodyLine returns a line of the render under the first line of a declaration, indented past
// the line number.
func bodyLine(indent, line string) string { return fmt.Sprintf("%s%7s%s\n", indent, "", line) }

// count returns the number of declarations in items, members included.
func count(items []Declaration) int {
	total := len(items)
	for _, item := range items {
		total += count(item.Members)
	}
	return total
}
