// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// DocumentInput is what an agent sends to the document tool.
//
// Doc is the documentation itself, as plain prose with no comment
// markers: which markers this language writes, how they are indented and
// where they go relative to the declaration are what the tool is for.
// Name and Kind pick out the declaration, by the rule every tool here
// addresses by. DryRun previews, and is what a caller gets when it says
// nothing.
type DocumentInput struct {
	Scope    string `json:"scope"              jsonschema:"file or directory, workspace-relative"`
	Name     string `json:"name"               jsonschema:"the declaration to document, qualified as the language writes it"`
	Doc      string `json:"doc"                jsonschema:"the documentation, as prose without comment markers"`
	Kind     string `json:"kind,omitempty"     jsonschema:"narrows an ambiguous name to one kind"`
	Language string `json:"language,omitempty" jsonschema:"language to assume"`
	DryRun   *bool  `json:"dry_run,omitempty"  jsonschema:"preview without writing; true when omitted"`
}

// DocumentOutput is what the document tool returns.
//
// It carries the same evidence a read answer does, plus what the gate
// made of the result. Applied is the one field a caller has to read:
// everything else describes a change that either did or did not happen.
type DocumentOutput struct {
	Scope   Scope  `json:"scope"`
	Symbol  string `json:"symbol,omitempty"`
	Applied bool   `json:"applied"`
	// Items are the files the change touches, one entry each.
	Items []Changed `json:"items"`
	// Verified is what judged the result, and is absent when nothing
	// could.
	Verified   *Gate      `json:"verified,omitempty"`
	Provenance Provenance `json:"provenance"`
	Error      *Failure   `json:"error,omitempty"`
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
func (o DocumentOutput) Failed() bool { return o.Error != nil }

// Render writes the change for a reader rather than a parser.
//
// A diff, because a change is read as one: what goes marked with a minus
// and what arrives with a plus. The last line says what to call next,
// because a preview whose gate passed is one call away from being real.
func (o DocumentOutput) Render() string {
	var b strings.Builder

	state := "preview"
	switch {
	case o.Error != nil:
		state = o.Error.Code
	case o.Applied:
		state = "applied"
	}
	fmt.Fprintf(&b, "document %s — %s\n", o.Symbol, state)

	if o.Error != nil {
		fmt.Fprintf(&b, "%s\n", o.Error.Reason)
		return b.String()
	}

	for _, item := range o.Items {
		for _, one := range item.Changes {
			fmt.Fprintf(&b, "\n%s:%d\n", item.Path, one.Line)
			for _, line := range diffed(one) {
				fmt.Fprintf(&b, "%s\n", line)
			}
		}
	}

	b.WriteString("\n")
	switch {
	case o.Verified == nil:
		b.WriteString("nothing checked this language")
	case o.Verified.Result == "pass":
		fmt.Fprintf(&b, "%ss (%s)", o.Verified.Gate, o.Verified.Engine)
	default:
		fmt.Fprintf(&b, "%s: %s (%s)", o.Verified.Gate, o.Verified.Result, o.Verified.Engine)
	}
	if !o.Applied {
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

// lines splits text for a diff, and returns nothing for text that is
// not there. A trailing newline is punctuation between lines rather
// than a line of its own.
func lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// Document builds the tool that writes documentation onto a
// declaration.
func Document(reads Outliner, writes Writer) (Tool, error) {
	return New(string(edit.DocumentSymbol), documentDescription,
		func(ctx context.Context, in DocumentInput) (DocumentOutput, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return DocumentOutput{}, err
			}
			held := Scope{Language: in.Language, Unit: string(scope)}
			if names(scope) {
				held.Path, held.Unit = string(scope), ""
			}
			if in.Doc == "" {
				return declined(held, in.Name, "refused",
					"there is no documentation to write: doc is the prose to put on the declaration"), nil
			}

			answered, err := reads.Outline(ctx, engine.Request{
				Scope:    scope,
				Language: source.Language(in.Language),
				Tests:    true,
			})
			if err != nil {
				return DocumentOutput{}, err
			}
			if !answered.Status.Answered() {
				return declined(held, in.Name, "unsupported",
					fmt.Sprintf("nothing outlines %q, so no declaration could be found in it", scope)), nil
			}

			found, failure := pick(answered.Items, scope, in.Name, kindOf(in.Kind))
			if failure != nil {
				return declined(held, in.Name, failure.Code, failure.Reason), nil
			}
			held.Language = string(found.Language)

			done, err := writes.Apply(ctx, edit.Request{
				Operation: edit.DocumentSymbol,
				Scope:     scope,
				Language:  found.Language,
				Target:    edit.Target{Kind: edit.TargetSpan, Span: found.Span},
				Args:      edit.Args{edit.ArgDoc: in.Doc},
				DryRun:    in.DryRun == nil || *in.DryRun,
			})
			if err != nil {
				return DocumentOutput{}, err
			}
			return reported(held, in.Name, done), nil
		})
}

// reported turns what the write path did into what a caller reads.
func reported(scope Scope, name string, done edit.Outcome) DocumentOutput {
	out := DocumentOutput{
		Scope:      scope,
		Symbol:     name,
		Applied:    done.Applied,
		Items:      touched(done.Rewrites),
		Provenance: provenance(done.Provenance),
	}

	switch {
	case done.Status == trust.Refused && len(done.Diagnostics) > 0:
		out.Verified = &Gate{
			Gate: "parse", Engine: done.Provenance.Engine,
			Result: done.Diagnostics[0].Diagnostic.Message,
		}
		out.Error = &Failure{Code: "refused", Reason: done.Reason}
	case done.Status == trust.Refused:
		out.Error = &Failure{Code: "refused", Reason: done.Reason}
	case done.Status == trust.Unsupported:
		out.Error = &Failure{Code: "unsupported", Reason: done.Reason}
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

// declined is the answer when the request never reached the write path.
func declined(scope Scope, name, code, why string) DocumentOutput {
	return DocumentOutput{
		Scope:  scope,
		Symbol: name,
		Items:  []Changed{},
		Error:  &Failure{Code: code, Reason: why},
	}
}

const documentDescription = "PREFER OVER editing a file to add a doc comment. " +
	"Writes documentation onto one declaration in the form that language's own " +
	"documentation tool reads: /// for Rust, /** */ for Java, a docstring inside the body " +
	"for Python, and at the declaration's own indentation. Send the prose only, with no " +
	"comment markers. Documentation already there is replaced. Previews by default and " +
	"refuses a change that stops the file parsing, so applying is a second call with " +
	"dry_run false."
