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

// Written is what every write tool returns.
//
// One shape for every operation, so an agent that has read one change
// can read the next without learning a second answer. Applied is the
// one field a caller has to read; everything else describes a change
// that either did or did not happen.
type Written struct {
	Scope     Scope  `json:"scope"`
	Operation string `json:"operation"`
	// Target is what the change was about: a declaration for the
	// operations pointed at one, a path for the operations pointed at a
	// file.
	Target  string `json:"target,omitempty"`
	Applied bool   `json:"applied"`
	// Items are the files the change touches, one entry each.
	Items []Changed `json:"items"`
	// Verified is what judged the result, and is absent when nothing
	// could. A change nothing gated is not a change that passed.
	Verified   *Gate      `json:"verified,omitempty"`
	Provenance Provenance `json:"provenance"`
	// Handle applies this change without planning it again, and is
	// present on a preview that could be applied.
	Handle string   `json:"handle,omitempty"`
	Error  *Failure `json:"error,omitempty"`
}

// Changed is one file a change touches.
//
// Every file it touches, not only the ones whose text it rewrites. A
// move rewrites nothing and still changes two paths, and an answer that
// said it had applied while naming no file would leave a caller unable
// to say what happened.
type Changed struct {
	Path string `json:"path"`
	// To is where the file goes, on a change that moves it.
	To string `json:"to,omitempty"`
	// Gone reports a file the change takes away.
	Gone bool `json:"gone,omitempty"`
	// Made reports a file the change creates.
	Made bool `json:"made,omitempty"`
	// Sites is how many ranges within the file are rewritten.
	Sites   int       `json:"sites"`
	Changes []Rewrite `json:"changes,omitempty"`
}

// mended is the fixes a gate offered, as text rather than as offsets.
//
// The offsets are into content nobody has written, so handing them over
// would be handing over coordinates a caller cannot use. What it can use
// is what to write.
func mended(found []edit.Finding) []Fix {
	var out []Fix
	for _, one := range found {
		for _, change := range one.Fix {
			for _, held := range change.Edits {
				out = append(out, Fix{
					Path: string(change.Path),
					Line: one.Diagnostic.Span.Start.Line + 1,
					Now:  held.New,
				})
			}
		}
	}
	return out
}

// Rewrite is one range within a file, as text rather than as offsets.
type Rewrite struct {
	Line int    `json:"line"`
	Was  string `json:"was,omitempty"`
	Now  string `json:"now"`
}

// Gate is what judged a change before it was written.
//
// It names what ran as well as what it found. "It parses" and "it
// compiles" are different promises, and a caller told only "pass" cannot
// tell which one it was given — nor which engine gave it, which is not
// the one that planned the change: a parser gates what a type checker
// planned whenever no server is running.
type Gate struct {
	Gate   string `json:"gate"`
	Engine string `json:"engine,omitempty"`
	Result string `json:"result"`
	// Fixes are what the gate said would resolve what it found, where it
	// named one obvious change each. They are written against the result
	// the change would have produced rather than against the files as
	// they are, so a caller reads them and asks again rather than
	// applying them on their own.
	Fixes []Fix `json:"fixes,omitempty"`
}

// gated is what judged a change, and nothing where nothing did.
//
// The tier names what was checked. A syntactic gate says the result is
// still the language it was; a resolved one says it still means
// something, and only the second refuses a rename onto a name already
// taken.
func gated(done edit.Outcome, result string) *Gate {
	if done.Gate == nil {
		return nil
	}
	held := "parse"
	if done.Gate.Fidelity >= trust.Resolved {
		held = "compile"
	}
	return &Gate{
		Gate: held, Engine: done.Gate.Engine, Result: result,
		Fixes: mended(done.Diagnostics),
	}
}

// Failed reports whether a caller should read this as a failure.
func (w Written) Failed() bool { return w.Error != nil }

// Render writes the change for a reader rather than a parser.
//
// A diff, because a change is read as one: what goes marked with a minus
// and what arrives with a plus. The last line says what to call next,
// because a preview whose gate passed is one call away from being real.
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

// diffed writes one rewrite as the lines a reader compares.
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

// lines splits text for a diff, and returns nothing for text that is not
// there. A trailing newline is punctuation between lines rather than a
// line of its own.
func lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// asked runs one write operation through the same steps, whatever the
// operation.
//
// Resolving what to point at is the caller's, because that is the only
// part that differs: a declaration is found by name, a file is named
// outright, and a span is given. Everything after that is shared, so an
// operation added later cannot answer in a shape of its own.
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

// addressing turns a name into the position an operation is pointed at.
//
// A span rather than an identity, because a unit declaring two methods
// called Get satisfies one identity twice, and because pointing at a
// position is what a language server takes for the same operations.
func addressing(
	ctx context.Context,
	reads Outliner,
	scope source.Path,
	language, name, kind string,
) (sema.Symbol, edit.Target, *Failure) {
	found, failure := addressed(ctx, reads, engine.Request{
		Scope:    scope,
		Language: source.Language(language),
	}, scope, name, kindOf(kind))
	if failure != nil {
		return sema.Symbol{}, edit.Target{}, failure
	}
	return found, edit.Target{Kind: edit.TargetSpan, Span: found.Span}, nil
}

// reported turns what the write path did into what a caller reads.
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
		// Nothing judged it, so no gate is reported rather than one
		// reporting that it passed.
	default:
		out.Verified = gated(done, "pass")
	}
	return out
}

// applied is the files a change actually wrote, or nil where it wrote
// nothing because it was a preview.
//
// A preview describes what would happen and an applied change describes
// what did. The two differ where a plan's result is already there: the
// preview is right to show it and the applied answer is not, because
// nothing was written.
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

// Touched names every file a change touches, and puts the ranges it
// rewrites under the one they are in.
//
// The changes come first, so a file that is moved, made or taken away is
// named even though no text in it is rewritten. The rewrites then fill
// in what a reader compares.
//
// Where the change was applied, only what it wrote is named. A caller
// told a file changed acts on it, and a plan whose result was already
// there wrote nothing.
func Touched(changes []edit.Change, rewrites []edit.Rewrite, wrote map[string]bool) []Changed {
	out := []Changed{}
	at := map[string]int{}

	held := func(p string) int {
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
		i := held(string(c.Path))
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
		i := held(string(r.Path))
		out[i].Sites++
		out[i].Changes = append(out[i].Changes, Rewrite{Line: r.Line, Was: r.Was, Now: r.Now})
	}
	return out
}

// declined is the answer when a request never reached the write path.
func declined(op edit.Operation, scope Scope, target, code, why string) Written {
	return Written{
		Scope:     scope,
		Operation: string(op),
		Target:    target,
		Items:     []Changed{},
		Error:     &Failure{Code: code, Reason: why},
	}
}

// about names what a write is about, so no item has to.
func writing(scope source.Path, language string) Scope {
	held := Scope{Language: language, Unit: string(scope)}
	if names(scope) {
		held.Path, held.Unit = string(scope), ""
	}
	return held
}

// previewing reports whether a caller asked to write.
//
// Silence is a preview. Writing by default would make a mistyped call a
// change to the workspace, and the second call costs one turn.
func previewing(asked *bool) bool { return asked == nil || *asked }
