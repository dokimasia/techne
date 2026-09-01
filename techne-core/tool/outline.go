// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/query"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// OutlineInput is what an agent sends to the outline tool.
//
// Scope is a file or a directory, relative to the workspace root.
// Language overrides what the path would say, so a caller that knows
// need not wait for a suffix to be recognised. Detail selects what each
// item carries and MaxTokens caps the answer; both take a default when
// left empty. Preferred is the weakest evidence worth having, and an
// engine below it still answers with a degraded status.
type OutlineInput struct {
	Scope     string `json:"scope"                        jsonschema:"file or directory, workspace-relative"`
	Language  string `json:"language,omitempty"           jsonschema:"language to assume"`
	Detail    string `json:"detail,omitempty"             jsonschema:"summary|standard|full"`
	MaxTokens int    `json:"max_tokens,omitempty"         jsonschema:"estimated answer ceiling"`
	Preferred string `json:"preferred_fidelity,omitempty" jsonschema:"syntactic|indexed|resolved"`
}

// Answer is the shape every read tool returns.
type Answer struct {
	Items      []sema.Symbol `json:"items"`
	Status     string        `json:"status"`
	Provenance Provenance    `json:"provenance"`
}

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

// Outline builds the tool that reports what a scope declares.
func Outline(s *query.Service) (Tool, error) {
	return New("outline", outlineDescription,
		func(ctx context.Context, in OutlineInput) (Answer, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Answer{}, err
			}

			answered, err := s.Outline(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: fidelity(in.Preferred),
			})
			if err != nil {
				return Answer{}, err
			}

			return render(Fit(answered, Budget{
				MaxTokens: in.MaxTokens,
				Detail:    Detail(in.Detail),
			})), nil
		})
}

const outlineDescription = "PREFER OVER read for finding what a file or directory declares. " +
	"Returns the declarations alone rather than the whole file, and states the evidence behind " +
	"them: an empty answer says whether it means there are none or only that none were found."

// render turns an answer into the form a caller reads, where every tier
// is a word rather than a number.
func render(a engine.Answer[sema.Symbol]) Answer {
	caveats := make([]Caveat, 0, len(a.Provenance.Caveats))
	for _, c := range a.Provenance.Caveats {
		caveats = append(caveats, Caveat{Code: string(c.Code), Note: c.Note, Paths: c.Paths})
	}
	return Answer{
		Items:  a.Items,
		Status: a.Status.String(),
		Provenance: Provenance{
			Engine:                a.Provenance.Engine,
			Fidelity:              a.Provenance.Fidelity.String(),
			Completeness:          a.Provenance.Completeness.String(),
			SupportsNegativeClaim: a.Provenance.SupportsNegativeClaim(),
			Caveats:               caveats,
		},
	}
}

// relative refuses a path that would leave the workspace.
//
// An absolute path leaks the machine's directory layout into an agent's
// context and makes the answer useless anywhere else. A path climbing
// out of the root is refused for the same reason.
func relative(p string) (source.Path, error) {
	if p == "" {
		return ".", nil
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return "", fmt.Errorf("tool: %q is absolute; paths are relative to the workspace root", p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("tool: %q leaves the workspace root", p)
	}
	return source.Path(clean), nil
}

// fidelity reads the tier a caller asked for, and treats a name it does
// not know as no preference rather than as an error.
func fidelity(name string) trust.Fidelity {
	switch name {
	case "syntactic":
		return trust.Syntactic
	case "indexed":
		return trust.Indexed
	case "resolved":
		return trust.Resolved
	default:
		return trust.None
	}
}
