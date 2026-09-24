// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Written is what every write tool returns. Applied reports whether the change was written,
// and the other fields describe the change either way.
type Written struct {
	Scope     Scope  `json:"scope"`
	Operation string `json:"operation"`
	// Target is the declaration or the file that the change is about.
	Target  string `json:"target,omitempty"`
	Applied bool   `json:"applied"`
	// Items are the files that the change touches, one each.
	Items []Changed `json:"items"`
	// Verified is the gate that checked the change, or nil when no gate checked it.
	Verified *Gate `json:"verified,omitempty"`
	// Provenance is the evidence of the engine that planned the change.
	Provenance Provenance `json:"provenance"`
	// Handle applies a preview with the apply.change tool without planning it again. A
	// preview that cannot be applied has none.
	Handle string   `json:"handle,omitempty"`
	Error  *Failure `json:"error,omitempty"`
}

// Changed is one file that a change touches, including a file that it moves, creates or
// deletes without rewriting its text.
type Changed struct {
	Path string `json:"path"`
	// To is the destination of a file that the change moves.
	To string `json:"to,omitempty"`
	// Gone reports a file that the change deletes.
	Gone bool `json:"gone,omitempty"`
	// Made reports a file that the change creates.
	Made bool `json:"made,omitempty"`
	// Sites is the number of ranges of the file that the change rewrites.
	Sites   int       `json:"sites"`
	Changes []Rewrite `json:"changes,omitempty"`
}

// Rewrite is one range of a file that a change rewrites: the line on which it starts, counted
// from one, the text it replaces and the text it writes.
type Rewrite struct {
	Line int    `json:"line"`
	Was  string `json:"was,omitempty"`
	Now  string `json:"now"`
}

// Gate is the check that judged a change before the write: its kind, the engine that ran it,
// its result, its fixes and its caveats. A parse gate checks that the result parses, and a
// compile gate checks that the result type-checks. The engine of the gate can differ from the
// engine that planned the change.
type Gate struct {
	Gate   string `json:"gate"`
	Engine string `json:"engine,omitempty"`
	Result string `json:"result"`
	// Fixes are the remedies that the gate offers for what it found. They apply to the result
	// of the change, not to the files on disk.
	Fixes []Fix `json:"fixes,omitempty"`
	// Caveats are the limits of the check, such as the dependents of a change that it did not
	// check.
	Caveats []Caveat `json:"caveats,omitempty"`
}

// gated returns the gate of done with result, or nil when no gate checked done. A gate at
// [trust.Resolved] compiles the result, and a gate below it parses the result.
func gated(done edit.Outcome, result string) *Gate {
	if done.Gate == nil {
		return nil
	}
	kind := "parse"
	if done.Gate.Fidelity >= trust.Resolved {
		kind = "compile"
	}
	return &Gate{
		Gate:    kind,
		Engine:  done.Gate.Engine,
		Result:  result,
		Fixes:   mended(done.Diagnostics),
		Caveats: provenance(*done.Gate).Caveats,
	}
}

// mended returns the text that the fixes of found write, each at the line of its diagnostic.
// The offsets of a fix are offsets into the result of the change, which no file contains.
func mended(found []edit.Finding) []Fix {
	var out []Fix
	for _, one := range found {
		for _, change := range one.Fix {
			for _, e := range change.Edits {
				out = append(out, Fix{
					Path: string(change.Path),
					Line: one.Diagnostic.Span.Start.Line + 1,
					Now:  e.New,
				})
			}
		}
	}
	return out
}

// Failed reports whether the output has an Error.
func (w Written) Failed() bool { return w.Error != nil }

// Render returns the change as text: a heading with the operation, the target and the state,
// each file with its rewrites as a diff, the evidence of the plan, the gate with its caveats,
// and the call that applies a preview.
func (w Written) Render() string {
	var b strings.Builder

	state := "preview"
	switch {
	case w.Error != nil:
		state = w.Error.Code
	case w.Applied:
		state = "applied"
	}
	fmt.Fprintf(&b, "%s %s — %s\n", w.Operation, w.Target, state)
	if w.Error != nil {
		fmt.Fprintf(&b, "%s\n", w.Error.Reason)
		return b.String()
	}

	for _, item := range w.Items {
		switch {
		case item.To != "":
			fmt.Fprintf(&b, "\n%s → %s\n", item.Path, item.To)
		case item.Gone:
			fmt.Fprintf(&b, "\n%s removed\n", item.Path)
		case item.Made:
			fmt.Fprintf(&b, "\n%s made\n", item.Path)
		}
		for _, one := range item.Changes {
			fmt.Fprintf(&b, "\n%s:%d\n", item.Path, one.Line)
			for _, line := range diffed(one) {
				fmt.Fprintf(&b, "%s\n", line)
			}
		}
	}

	b.WriteString("\n")
	if w.Provenance.Fidelity != "" {
		// The negative claim is about an empty answer, and a plan is not one.
		plan := w.Provenance
		plan.SupportsNegativeClaim = false
		b.WriteString("plan: " + evidence(plan))
	}
	switch {
	case w.Verified == nil:
		b.WriteString("nothing checked this language")
	case w.Verified.Result == "pass":
		fmt.Fprintf(&b, "%ss (%s)", w.Verified.Gate, w.Verified.Engine)
	default:
		fmt.Fprintf(&b, "%s: %s (%s)", w.Verified.Gate, w.Verified.Result, w.Verified.Engine)
		for _, one := range w.Verified.Fixes {
			fmt.Fprintf(&b, "\n%s:%d would take %q", one.Path, one.Line, one.Now)
		}
	}
	if w.Verified != nil {
		for _, c := range w.Verified.Caveats {
			b.WriteString(". " + c.Note)
		}
	}
	switch {
	case w.Applied:
	case w.Handle != "":
		fmt.Fprintf(&b, ". apply with apply.change handle %s", w.Handle)
	default:
		b.WriteString(". apply by calling again with dry_run false")
	}
	b.WriteString("\n")
	return b.String()
}

// diffed returns the lines of one rewrite as a diff: each line that it replaces after a minus,
// and each line that it writes after a plus.
func diffed(one Rewrite) []string {
	var out []string
	for _, line := range lines(one.Was) {
		out = append(out, "- "+line)
	}
	for _, line := range lines(one.Now) {
		out = append(out, "+ "+line)
	}
	return out
}

// lines returns the lines of text without a final line ending, and nothing for empty text.
func lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// asked runs one write operation and returns what it did. A write tool resolves its target
// and builds req, and every write tool reports the outcome through [reported].
func asked(
	ctx context.Context,
	writes Writer,
	op edit.Operation,
	scope Scope,
	target string,
	req edit.Request,
) (Written, error) {
	done, err := writes.Apply(ctx, req)
	if err != nil {
		return Written{}, err
	}
	return reported(op, scope, target, done), nil
}

// addressing returns the declaration that a name and a kind address in scope, by the rule of
// [addressed], and the target of an operation at its span. The span identifies one of two
// declarations that share an ID, and a language server takes a position for the same
// operations.
func addressing(
	ctx context.Context,
	reads Outliner,
	scope source.Path,
	language, name string,
	word KindWord,
) (sema.Symbol, edit.Target, *Failure) {
	kind, failure := kindOf(word)
	if failure != nil {
		return sema.Symbol{}, edit.Target{}, failure
	}
	found, failure := addressed(ctx, reads, engine.Request{
		Scope:    scope,
		Language: source.Language(language),
	}, scope, name, kind)
	if failure != nil {
		return sema.Symbol{}, edit.Target{}, failure
	}
	return found, edit.Target{Kind: edit.TargetSpan, Span: found.Span}, nil
}

// reported returns the output of the write tool op for done. A refused or unsupported
// outcome has a [Failure]. A degraded outcome has no gate, because no gate checked the
// change, and any other outcome has the gate that passed it.
func reported(op edit.Operation, scope Scope, target string, done edit.Outcome) Written {
	out := Written{
		Scope:      scope,
		Operation:  string(op),
		Target:     target,
		Applied:    done.Applied,
		Items:      Touched(done.Changes, done.Rewrites, applied(done)),
		Handle:     done.Handle,
		Provenance: provenance(done.Provenance),
	}

	switch {
	case done.Status == trust.Refused && len(done.Diagnostics) > 0:
		out.Verified = gated(done, done.Diagnostics[0].Diagnostic.Message)
		out.Error = &Failure{Code: trust.Refused.String(), Reason: done.Reason}
	case done.Status == trust.Refused:
		out.Error = &Failure{Code: trust.Refused.String(), Reason: done.Reason}
	case done.Status == trust.Unsupported:
		out.Error = &Failure{Code: trust.Unsupported.String(), Reason: done.Reason}
	case done.Status == trust.Degraded:
	default:
		out.Verified = gated(done, "pass")
	}
	return out
}

// applied returns the files that done wrote, or nil for a preview. A plan whose result equals
// the content on disk writes no file.
func applied(done edit.Outcome) map[string]bool {
	if !done.Applied {
		return nil
	}
	out := map[string]bool{}
	for _, p := range done.Changed {
		out[string(p)] = true
	}
	return out
}

// Touched returns the files that a change touches, each with the rewrites in it. A file that
// the change moves, creates or deletes comes first from changes, and the rewrites follow
// under the file they are in. With wrote set, Touched returns only the files that wrote
// names, because an applied change reports what it wrote. With wrote nil, it returns every
// file of a preview.
func Touched(changes []edit.Change, rewrites []edit.Rewrite, wrote map[string]bool) []Changed {
	out := []Changed{}
	at := map[string]int{}

	entry := func(p string) int {
		i, seen := at[p]
		if !seen {
			i = len(out)
			at[p] = i
			out = append(out, Changed{Path: p})
		}
		return i
	}
	kept := func(p string) bool { return wrote == nil || wrote[p] }

	for _, c := range changes {
		if !kept(string(c.Path)) && !kept(string(c.To)) {
			continue
		}
		i := entry(string(c.Path))
		switch c.Kind {
		case edit.ChangeMove:
			out[i].To = string(c.To)
		case edit.ChangeDelete:
			out[i].Gone = true
		case edit.ChangeCreate:
			out[i].Made = true
		case edit.ChangeEdit, edit.ChangeUnset:
		}
	}
	for _, r := range rewrites {
		if !kept(string(r.Path)) {
			continue
		}
		i := entry(string(r.Path))
		out[i].Sites++
		out[i].Changes = append(out[i].Changes, Rewrite{Line: r.Line, Was: r.Was, Now: r.Now})
	}
	return out
}

// declined returns the output of a request that the tool refuses before the write path
// reads it.
func declined(op edit.Operation, scope Scope, target, code, why string) Written {
	return Written{
		Scope:     scope,
		Operation: string(op),
		Target:    target,
		Items:     []Changed{},
		Error:     &Failure{Code: code, Reason: why},
	}
}

// writing returns the scope of a write request about scope in language: the file for a scope
// that names one, and the unit otherwise.
func writing(scope source.Path, language string) Scope {
	out := Scope{Language: language, Unit: string(scope)}
	if names(scope) {
		out.Path, out.Unit = string(scope), ""
	}
	return out
}

// previewing reports whether a request writes nothing: dry is nil or true. A request that
// does not say is a preview.
func previewing(dry *bool) bool { return dry == nil || *dry }
