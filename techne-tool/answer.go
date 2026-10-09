// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"cmp"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Scope is what an answer is about: the language, the unit and the file of the request. An
// item states its own path only when it is not in the file of the scope, as in an answer about
// more than one file.
type Scope struct {
	Language string `json:"language,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Path     string `json:"path,omitempty"`
	// Directory is the directory of an answer about a directory of two or more units, which
	// states no unit.
	Directory string `json:"directory,omitempty"`
	// Summary is the first sentence of the documentation of the unit of an answer about a
	// directory of one unit, such as a Go package, or empty.
	Summary string `json:"summary,omitempty"`
}

// Answer is the output of the outline, search and resolve tools: the scope, the declarations,
// the evidence of the engine in Provenance, and an Error when no engine served the request.
// There is no status field.
type Answer struct {
	// Error is the reason that the request was not served, or nil.
	Error      *Failure      `json:"error,omitempty"`
	Scope      Scope         `json:"scope"`
	Items      []Declaration `json:"items"`
	Provenance Provenance    `json:"provenance"`
	// ByFile reports that the render writes the declarations of each file under a line with
	// its path, as the outline of a directory does. A ranked answer, such as a search, writes
	// the path of each declaration on its line instead. The render reads it, and the JSON of
	// the answer does not contain it.
	ByFile bool `json:"-"`
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
	Engine       string   `json:"engine,omitempty"`
	Fidelity     string   `json:"fidelity"`
	Completeness string   `json:"completeness"`
	Caveats      []Caveat `json:"caveats,omitempty"`
	// SupportsNegativeClaim reports whether an empty answer proves that there are none, which
	// needs resolved binding over total coverage.
	SupportsNegativeClaim bool `json:"supportsNegativeClaim"`
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

// published returns the answer of a read tool: items, the declarations of a as [Declared] or
// [Resolved] builds them, and the evidence of a. A declaration in the file of scope states no
// path, by the rule of [stated]. An answer that no engine served has a [Failure] whose reason is
// the first note of its caveats.
func published(a engine.Answer[sema.Symbol], scope Scope, items []Declaration) Answer {
	out := Answer{
		Scope:      scope,
		Items:      stated(items, scope.Path),
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
	return Answer{Scope: scope, Items: []Declaration{}, Provenance: unserved(), Error: f}
}

// unserved returns the provenance of a request that no engine served: the tiers none and
// unknown, as [trust.Provenance] states them at its zero value.
func unserved() Provenance { return provenance(trust.Provenance{}) }

// stated returns items without the path of each declaration in file, members included,
// because the scope of the answer states file. An answer about more than one file has no file
// in its scope, so every declaration keeps its path.
func stated(items []Declaration, file string) []Declaration {
	for i := range items {
		if items[i].Path == file {
			items[i].Path = ""
		}
		items[i].Members = stated(items[i].Members, file)
	}
	return items
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

// headed returns the heading of the render of an answer about scope with n declarations: the
// file, the unit or the directory, the language, the unit when the first part is not the unit,
// and the count, and then the summary of the unit on a line of its own.
func headed(scope Scope, n int) string {
	about := cmp.Or(scope.Path, scope.Unit, scope.Directory)
	unit := scope.Unit
	if unit == "." || unit == about {
		unit = ""
	}
	held := strings.TrimSpace(scope.Language + " " + unit)
	out := fmt.Sprintf("%s — %s, %s", about, held, plural(n, "declaration", "declarations"))
	if scope.Summary != "" {
		out += "\n" + scope.Summary
	}
	return out
}

// body writes the declarations of the answer and then its evidence. An answer without
// declarations writes none in their place. In an answer by file, a declaration that states its
// path writes the path on a line of its own before it, unless the declaration before it is in
// the same file, and then writes its line alone, as the declarations of the file of the scope
// do.
func (a Answer) body(b *strings.Builder, none string) {
	if len(a.Items) == 0 {
		b.WriteString(none + "\n")
	}
	file := ""
	for _, item := range a.Items {
		if a.ByFile && item.Path != "" && item.Path != file {
			file = item.Path
			b.WriteString(fileLine(file))
		}
		item.render(b, 0, file)
	}
	b.WriteString("\n")
	b.WriteString(evidence(a.Provenance, len(a.Items) == 0))
}

// evidence returns the last line of every render: the fidelity, the completeness, and the note
// of each caveat that is not [trust.CaveatDynamic]. The fidelity states the limit of a dynamic
// caveat, and the structured answer contains its note. An empty answer also states whether it
// proves that there are none, and the notes of its dynamic caveats, which state why it can be
// empty. Every tool renders its evidence with this function.
func evidence(p Provenance, empty bool) string {
	var b strings.Builder
	b.WriteString(p.Fidelity + ", " + p.Completeness + " coverage")
	if empty && p.SupportsNegativeClaim {
		b.WriteString(". an empty answer here means there are none")
	}
	for _, c := range p.Caveats {
		if empty || c.Code != string(trust.CaveatDynamic) {
			b.WriteString(". " + c.Note)
		}
	}
	b.WriteString(".\n")
	return b.String()
}

// plural returns n with the singular form one for a count of 1 and the plural form many for
// any other count.
func plural(n int, one, many string) string { return fmt.Sprintf("%d %s", n, noun(n, one, many)) }

// noun returns one for a count of 1 and many for any other count.
func noun(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// render writes d and then each of its members one level deeper. It writes all the
// documentation and all the source text of d. The documentation of a declaration with source
// text comes before the source text, as in the file, and otherwise after the signature. within
// is the path of the declaration that contains d, which d does not write again.
func (d Declaration) render(b *strings.Builder, depth int, within string) {
	indent := strings.Repeat("  ", depth)
	head, rest := parts(d)
	documented := d.Doc != ""
	if documented && d.Snippet != "" {
		d.docs(b, indent)
		documented = false
	}
	b.WriteString(headLine(indent, d, head, within))
	for _, line := range rest {
		b.WriteString(bodyLine(indent, line))
	}
	if documented {
		d.docs(b, indent)
	}
	for _, member := range d.Members {
		member.render(b, depth+1, cmp.Or(d.Path, within))
	}
}

// docs writes each line of the documentation of d under indent.
func (d Declaration) docs(b *strings.Builder, indent string) {
	for line := range strings.SplitSeq(d.Doc, "\n") {
		b.WriteString(bodyLine(indent, line))
	}
}

// parts returns the first line that the render of d writes and the lines of source text
// under it. Source text starts with the signature, so a declaration with source text writes
// the source text in place of the signature. A declaration without either writes its kind and
// its name. A declaration without source text then writes the number of its members by kind,
// and a declaration with a summary writes the summary last.
func parts(d Declaration) (string, []string) {
	head, rest := d.Signature, []string(nil)
	if d.Snippet != "" {
		lines := strings.Split(d.Snippet, "\n")
		head, rest = lines[0], lines[1:]
	}
	if head == "" {
		head = d.Kind.String() + " " + d.Name
	}
	if counted := tally(d.Members); counted != "" && d.Snippet == "" {
		head += "  " + counted
	}
	if d.Summary != "" {
		head += "  " + d.Summary
	}
	return head, rest
}

// tally returns the number of members of each kind, in the order of [sema.Kinds], as
// "2 fields, 1 method", or the empty string for no member.
func tally(members []Declaration) string {
	counts := map[sema.Kind]int{}
	for _, one := range members {
		counts[one.Kind]++
	}
	var out []string
	for _, kind := range sema.Kinds() {
		if n := counts[kind]; n > 0 {
			word := kind.String()
			out = append(out, plural(n, word, cmp.Or(irregular[word], word+"s")))
		}
	}
	return strings.Join(out, ", ")
}

// irregular are the plural forms of the words of [sema.Kinds] that do not add an s.
var irregular = map[string]string{"property": "properties"}

// headLine returns the first line of the render of d: its path and its line when d states a
// path other than within, its line otherwise, and then head.
func headLine(indent string, d Declaration, head, within string) string {
	if d.Path != "" && d.Path != within {
		return fmt.Sprintf("%s%s:%d  %s\n", indent, d.Path, d.Line, head)
	}
	return fmt.Sprintf("%s%5d  %s\n", indent, d.Line, head)
}

// fileLine returns the line of the render that names the file of the declarations after it.
func fileLine(path string) string { return path + "\n" }

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
