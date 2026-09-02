// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
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
// left empty. Names, Kind and Prefix narrow the answer to what the
// caller meant, which is what makes the levels carrying documentation
// and source text worth asking for. Preferred is the weakest evidence
// worth having, and an engine below it still answers.
type OutlineInput struct {
	Scope     string   `json:"scope"                        jsonschema:"file or directory, workspace-relative"`
	Language  string   `json:"language,omitempty"           jsonschema:"language to assume"`
	Detail    string   `json:"detail,omitempty"             jsonschema:"names|signatures|docs|source"`
	Names     []string `json:"names,omitempty"              jsonschema:"limit the answer to these declarations"`
	Kind      string   `json:"kind,omitempty"               jsonschema:"limit the answer to one kind"`
	Prefix    string   `json:"prefix,omitempty"             jsonschema:"limit to names starting with this"`
	Private   bool     `json:"private,omitempty"            jsonschema:"include declarations not visible outside their unit"`
	Include   []string `json:"include,omitempty"            jsonschema:"import|parameter|local|all"`
	Tests     bool     `json:"tests,omitempty"              jsonschema:"include the files this language calls tests"`
	MaxTokens int      `json:"max_tokens,omitempty"         jsonschema:"estimated answer ceiling"`
	Preferred string   `json:"preferred_fidelity,omitempty" jsonschema:"syntactic|indexed|resolved"`
}

// Outline builds the tool that reports what a scope declares.
func Outline(reads Outliner) (Tool, error) {
	return New("outline", outlineDescription,
		func(ctx context.Context, in OutlineInput) (Answer, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Answer{}, err
			}

			answered, err := reads.Outline(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: fidelity(in.Preferred),
				Tests:     in.Tests,
			})
			if err != nil {
				return Answer{}, err
			}

			detail := level(in.Detail, scope)
			out := published(answered, about(scope, in.Language, answered), detail, in.Include)
			out.Items = Narrow{
				Names: in.Names, Kind: kindOf(in.Kind),
				Prefix: in.Prefix, Private: in.Private,
			}.Apply(out.Items)
			return Fit(out, Budget{MaxTokens: in.MaxTokens}), nil
		})
}

const outlineDescription = "PREFER OVER read for finding what a file or directory declares: " +
	"about a quarter of the tokens, measured on real files in Go, Python, Java and TypeScript. " +
	"Returns declarations rather than lines, and states the evidence behind them, so an empty " +
	"answer says whether it means there are none or only that none were found. " +
	"The docs and source levels return whole comments and whole bodies, so over a whole file " +
	"they cost more than reading it: narrow them with names, kind or prefix."

// about names what an answer is about, so no item has to.
//
// The language is what the engine answered as, which is what a caller
// asked about only when it said so. A scope naming a file states the
// file; a directory leaves it to the items, which come from several.
func about(scope source.Path, asked string, a engine.Answer[sema.Symbol]) Scope {
	held := Scope{Language: asked}
	if len(a.Items) > 0 {
		held.Language = string(a.Items[0].Language)
	}
	if names(scope) {
		held.Path = string(scope)
		held.Unit = path.Dir(string(scope))
		return held
	}
	held.Unit = string(scope)
	return held
}

// names reports whether a scope names one file.
//
// The suffix is not enough on its own: path.Ext reads "." as an
// extension of ".", so the workspace root would be taken for a file and
// answered at the level a file is answered at.
func names(scope source.Path) bool {
	base := path.Base(string(scope))
	suffix := path.Ext(base)
	return suffix != "" && suffix != base
}

// level reads the detail a caller asked for, and falls back to what the
// scope implies rather than to one answer for every scope.
func level(named string, scope source.Path) Detail {
	for _, d := range Levels() {
		if string(d) == named {
			return d
		}
	}
	return DefaultDetail(scope)
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
