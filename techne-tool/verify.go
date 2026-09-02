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
)

// VerifyInput is what an agent sends to the verify tool.
//
// Suites names what to run in the language's own words: its linters, its
// test runner, its type checker. Empty runs whatever that language runs
// by default, which is what a caller checking its own work wants.
type VerifyInput struct {
	Scope     string   `json:"scope"                        jsonschema:"file or directory, workspace-relative"`
	Suites    []string `json:"suites,omitempty"             jsonschema:"what to run, in the language's own words"`
	Language  string   `json:"language,omitempty"           jsonschema:"language to assume"`
	MaxIssues int      `json:"max_issues,omitempty"         jsonschema:"cap the issues returned"`
	Preferred string   `json:"preferred_fidelity,omitempty" jsonschema:"syntactic|indexed|resolved"`
}

// VerifyOutput is what the verify tool returns.
type VerifyOutput struct {
	Scope      Scope      `json:"scope"`
	Items      []Reported `json:"items"`
	Provenance Provenance `json:"provenance"`
	Error      *Failure   `json:"error,omitempty"`
}

// Reported is one thing a gate said about the code.
//
// At is the line the diagnostic is about, carried because whoever reads
// this has no filesystem and a message without its line costs a read
// each. Fix is the change that resolves it, and carries only what it
// would write: saying what it would replace needs the file.
type Reported struct {
	Severity string `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	At       string `json:"at,omitempty"`
	Fix      []Fix  `json:"fix,omitempty"`
}

// Fix is one range a diagnostic's remedy would write.
type Fix struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Now  string `json:"now"`
}

// Failed reports whether a caller should read this as a failure.
//
// Issues are not a failure. A gate that ran and found twelve problems
// answered the question it was asked, and a caller that treats that as a
// fault cannot act on the twelve.
func (o VerifyOutput) Failed() bool { return o.Error != nil }

// Render writes what the gate said for a reader rather than a parser.
func (o VerifyOutput) Render() string {
	var b strings.Builder
	if o.Error != nil {
		fmt.Fprintf(&b, "%s: %s\n", o.Error.Code, o.Error.Reason)
		return b.String()
	}

	about := o.Scope.Path
	if about == "" {
		about = o.Scope.Unit
	}
	fmt.Fprintf(&b, "%s — %s\n", about, plural(len(o.Items), "issue", "issues"))

	for _, one := range o.Items {
		fmt.Fprintf(&b, "\n%s:%d  %s", one.Path, one.Line, one.Severity)
		if one.Code != "" {
			fmt.Fprintf(&b, "  %s", qualify(one.Source, one.Code))
		}
		fmt.Fprintf(&b, "\n  %s\n", one.Message)
		if one.At != "" {
			fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(one.At))
		}
		if len(one.Fix) > 0 {
			b.WriteString("  fix available\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(evidence(o.Provenance))
	return b.String()
}

// Verify builds the tool that reports what a language's gate says.
func Verify(gates Verifier) (Tool, error) {
	return New("verify", verifyDescription,
		func(ctx context.Context, in VerifyInput) (VerifyOutput, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return VerifyOutput{}, err
			}

			answered, err := gates.Verify(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: fidelity(in.Preferred),
			}, in.Suites)
			if err != nil {
				return VerifyOutput{}, err
			}

			out := VerifyOutput{
				Scope:      Scope{Language: in.Language, Unit: string(scope)},
				Items:      reportedIn(answered.Items, in.MaxIssues),
				Provenance: provenance(answered.Provenance),
			}
			if names(scope) {
				out.Scope.Path, out.Scope.Unit = string(scope), ""
			}
			if !answered.Status.Answered() {
				out.Error = &Failure{
					Code:   answered.Status.String(),
					Reason: reasonFrom(out.Provenance.Caveats),
				}
			}
			return out, nil
		})
}

// reportedIn turns findings into what a caller reads, stopping at the
// cap it set.
func reportedIn(found []edit.Finding, limit int) []Reported {
	out := []Reported{}
	for _, one := range found {
		if limit > 0 && len(out) == limit {
			break
		}
		out = append(out, Reported{
			Severity: one.Diagnostic.Severity.String(),
			Code:     one.Diagnostic.Code,
			Source:   one.Diagnostic.Source,
			Message:  one.Diagnostic.Message,
			Path:     string(one.Diagnostic.Span.Path),
			Line:     one.Diagnostic.Span.Start.Line + 1,
			At:       one.Diagnostic.Snippet,
			Fix:      fixes(one.Fix),
		})
	}
	return out
}

// fixes reads a remedy as the lines it would write.
//
// Only what arrives, not what goes: reading the text a fix replaces
// needs the file, and nothing here has one. A caller that wants the
// before applies the fix as a preview.
func fixes(changes []edit.Change) []Fix {
	var out []Fix
	for _, c := range changes {
		for _, e := range c.Edits {
			out = append(out, Fix{
				Path: string(c.Path),
				Line: e.Span.Start.Line + 1,
				Now:  e.New,
			})
		}
	}
	return out
}

const verifyDescription = "PREFER OVER running the build or the linter in a shell. " +
	"Runs a language's own gate over a scope and returns what it said, each issue with " +
	"the line it is about and, where there is one obvious remedy, the change that makes " +
	"it. Finding issues is an answer rather than a failure. It is the same gate the write " +
	"path runs, so a caller can check its own work before asking for a change."
