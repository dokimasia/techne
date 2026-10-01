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

// VerifyInput is the input of the verify tool. Suites names the checks in the words of the
// language, such as its linters or its test runner, and the empty list runs the checks that the
// language runs by default.
type VerifyInput struct {
	Scope     string       `json:"scope"                        jsonschema:"file or directory, relative to the root"`
	Suites    []string     `json:"suites,omitempty"             jsonschema:"checks to run, in the words of the language"`
	Language  string       `json:"language,omitempty"           jsonschema:"a language to ask instead of the scope's"`
	MaxIssues int          `json:"max_issues,omitempty"         jsonschema:"most issues to return, all by default"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence wanted; a weaker answer is marked degraded"`
}

// VerifyOutput is the output of the verify tool.
type VerifyOutput struct {
	Scope      Scope      `json:"scope"`
	Items      []Reported `json:"items"`
	Provenance Provenance `json:"provenance"`
	Error      *Failure   `json:"error,omitempty"`
}

// Reported is one issue that a check reported. At is the source line of the issue. Fix is the
// text that the one obvious remedy writes, without the text it replaces, which only the file
// contains.
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

// Fix is one range that a remedy writes: its file, the line on which it starts, counted from
// one, and the text.
type Fix struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Now  string `json:"now"`
}

// Failed reports whether the output has an Error. Issues are no failure: a check that ran and
// reported issues served the request.
func (o VerifyOutput) Failed() bool { return o.Error != nil }

// Render returns the issues as text: a heading, each issue with its site, its severity, its
// code, its message, its source line and whether a fix is available, and the evidence.
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
			fmt.Fprintf(&b, "  %s", coded(one.Source, one.Code))
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
	b.WriteString(evidence(o.Provenance, len(o.Items) == 0))
	return b.String()
}

// coded returns the code of an issue after the tool that reported it and a dot, or the code
// alone when no tool is named.
func coded(reporter, code string) string {
	if reporter == "" {
		return code
	}
	return reporter + "." + code
}

// Verify returns the tool that runs the checks of a language over a scope and reports what
// they found. It returns the first MaxIssues issues, with a truncation caveat when it leaves
// any out.
func Verify(gates Verifier) (Tool, error) {
	return New("verify", verifyDescription,
		func(ctx context.Context, in VerifyInput) (VerifyOutput, error) {
			scope, failure := relative(in.Scope)
			out := VerifyOutput{
				Scope: Scope{Language: in.Language, Unit: string(scope)}, Items: []Reported{}, Provenance: unserved(),
			}
			if failure != nil {
				out.Scope.Unit, out.Error = "", failure
				return out, nil
			}
			if names(scope) {
				out.Scope.Path, out.Scope.Unit = string(scope), ""
			}
			preferred, failure := fidelityOf(in.Preferred)
			if failure != nil {
				out.Error = failure
				return out, nil
			}

			answered, err := gates.Verify(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: preferred,
			}, in.Suites)
			if err != nil {
				return VerifyOutput{}, err
			}

			out.Items = reportedIn(answered.Items, in.MaxIssues)
			out.Provenance = provenance(answered.Provenance)
			if len(out.Items) < len(answered.Items) {
				out.Provenance.Caveats = append(out.Provenance.Caveats, Caveat{
					Code: string(trust.CaveatTruncated),
					Note: fmt.Sprintf("%d of %d issues returned", len(out.Items), len(answered.Items)),
				})
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

// reportedIn returns the first limit findings of found, each as a caller reads it, and every
// finding for a limit of zero or less.
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

// fixes returns the ranges that changes write, each with the line on which it starts.
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
	"It runs the checks of a language over a scope and returns each issue with its line and, " +
	"where one is obvious, its fix."
