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
type Changed struct {
	Path    string    `json:"path"`
	Sites   int       `json:"sites"`
	Changes []Rewrite `json:"changes,omitempty"`
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
// builds" are different promises, and a caller told only "pass" cannot
// tell which one it was given.
type Gate struct {
	Gate   string `json:"gate"`
	Engine string `json:"engine,omitempty"`
	Result string `json:"result"`
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
		Items:      touched(done.Rewrites),
		Handle:     done.Handle,
		Provenance: provenance(done.Provenance),
	}

	switch {
	case done.Status == trust.Refused && len(done.Diagnostics) > 0:
		out.Verified = &Gate{
			Gate: "parse", Engine: done.Provenance.Engine,
			Result: done.Diagnostics[0].Diagnostic.Message,
		}
		out.Error = &Failure{Code: trust.Refused.String(), Reason: done.Reason}
	case done.Status == trust.Refused:
		out.Error = &Failure{Code: trust.Refused.String(), Reason: done.Reason}
	case done.Status == trust.Unsupported:
		out.Error = &Failure{Code: trust.Unsupported.String(), Reason: done.Reason}
	case done.Status == trust.Degraded:
		// Nothing judged it, so no gate is reported rather than one
		// reporting that it passed.
	default:
		out.Verified = &Gate{Gate: "parse", Engine: done.Provenance.Engine, Result: "pass"}
	}
	return out
}

// touched groups the ranges a change rewrites by the file they are in.
func touched(rewrites []edit.Rewrite) []Changed {
	out := []Changed{}
	at := map[string]int{}
	for _, r := range rewrites {
		i, held := at[string(r.Path)]
		if !held {
			i = len(out)
			at[string(r.Path)] = i
			out = append(out, Changed{Path: string(r.Path)})
		}
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
