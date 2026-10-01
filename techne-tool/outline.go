// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// OutlineInput is the input of the outline tool.
type OutlineInput struct {
	Scope     string       `json:"scope"                        jsonschema:"file or directory, relative to the root"`
	Language  string       `json:"language,omitempty"           jsonschema:"a language to ask instead of the scope's"`
	Detail    Detail       `json:"detail,omitempty"             jsonschema:"signatures for a file and names for a directory by default"`
	Names     []string     `json:"names,omitempty"              jsonschema:"keep these names"`
	Kind      KindWord     `json:"kind,omitempty"               jsonschema:"keep one kind, such as function, method, struct or interface"`
	Prefix    string       `json:"prefix,omitempty"             jsonschema:"keep the names that start with this"`
	Private   bool         `json:"private,omitempty"            jsonschema:"include unexported declarations"`
	Include   []Include    `json:"include,omitempty"            jsonschema:"bindings to add"`
	Tests     bool         `json:"tests,omitempty"              jsonschema:"include test files"`
	MaxTokens int          `json:"max_tokens,omitempty"         jsonschema:"answer ceiling in tokens, 6000 by default"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence wanted; a weaker answer is marked degraded"`
}

// Outline returns the tool that lists the declarations of a scope, by file. It narrows the
// answer by the names, the kind, the prefix and the visibility of the input, and fits it to the
// budget with [Fit]. An answer about a directory of one unit states the summary of the unit.
func Outline(reads Outliner) (Tool, error) {
	return New("outline", outlineDescription,
		func(ctx context.Context, in OutlineInput) (Answer, error) {
			scope, failure := relative(in.Scope)
			if failure != nil {
				return failed(Scope{Language: in.Language}, failure), nil
			}
			kind, byKind := kindOf(in.Kind)
			detail, byDetail := levelOf(in.Detail, scope)
			include, byInclude := bindingsOf(in.Include)
			preferred, byFidelity := fidelityOf(in.Preferred)
			if failure = first(byKind, byDetail, byInclude, byFidelity); failure != nil {
				return failed(about(scope, in.Language, engine.Answer[sema.Symbol]{}), failure), nil
			}

			answered, err := reads.Outline(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: preferred,
				Tests:     in.Tests,
			})
			if err != nil {
				return Answer{}, err
			}

			out := published(answered, about(scope, in.Language, answered), Declared(answered.Items, detail, include))
			out.ByFile = true
			if out.Scope.Path == "" && out.Scope.Unit != "" {
				out.Scope.Summary = unitSummary(answered.Items)
			}
			out.Items = Narrow{
				Names: in.Names, Kind: kind,
				Prefix: in.Prefix, Private: in.Private,
			}.Apply(out.Items)
			return Fit(out, Budget{MaxTokens: in.MaxTokens}), nil
		})
}

const outlineDescription = "PREFER OVER read for finding what a file or directory declares. " +
	"It returns the declarations with their signatures for about a quarter of the tokens of " +
	"the file. The summaries level adds the first sentence of each doc comment. The docs and " +
	"source levels cost more than the file, so narrow them with names, kind or prefix."

// about returns the scope of an answer about scope. The language is the language of the
// declarations of a when they share one, and empty when they are of more than one language.
// Without declarations it is asked, or else the language of the engine that published a. A
// scope that names a file states the file and its unit. A directory is the unit when its
// declarations are of one unit, and an answer about a directory of two or more units states the
// directory and no unit.
func about(scope source.Path, asked string, a engine.Answer[sema.Symbol]) Scope {
	out := Scope{Language: cmp.Or(asked, string(a.Language))}
	for i, s := range a.Items {
		if i == 0 {
			out.Language = string(s.Language)
		} else if string(s.Language) != out.Language {
			out.Language = ""
			break
		}
	}
	if names(scope) {
		out.Path = string(scope)
		out.Unit = path.Dir(string(scope))
		return out
	}
	out.Unit = string(scope)
	for _, s := range a.Items {
		if s.ID.Unit() != a.Items[0].ID.Unit() {
			out.Unit, out.Directory = "", string(scope)
			break
		}
	}
	return out
}

// names reports whether scope names one file: its last element has an extension and is more
// than the extension. path.Ext returns "." for ".", so the workspace root names no file.
func names(scope source.Path) bool {
	base := path.Base(string(scope))
	suffix := path.Ext(base)
	return suffix != "" && suffix != base
}

// filed returns a refusal for a path that [names] reports as no file, for a field of a tool
// that takes one file.
func filed(field string, p source.Path) *Failure {
	if names(p) {
		return nil
	}
	return &Failure{
		Code:   trust.Refused.String(),
		Reason: fmt.Sprintf("%s %q names a directory, and %s names one file", field, p, field),
	}
}

// relative returns p as a path of the workspace, and the root for the empty path. A backslash
// inside a path is a character of a file name. It returns a refused [Failure] for an absolute
// path of any platform, and for a path that is .. or starts with ../ after [path.Clean].
func relative(p string) (source.Path, *Failure) {
	if p == "" {
		return engine.Root, nil
	}
	if absolute(p) {
		return "", &Failure{
			Code:   trust.Refused.String(),
			Reason: fmt.Sprintf("%q is absolute, not relative to the workspace root", p),
		}
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", &Failure{Code: trust.Refused.String(), Reason: fmt.Sprintf("%q leaves the workspace root", p)}
	}
	return source.Path(clean), nil
}

// absolute reports whether p is absolute on some platform: it starts with a slash or a
// backslash, as a Unix path and a Windows UNC path do, or with a drive letter and a colon.
func absolute(p string) bool {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return true
	}
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	letter := p[0] | 0x20
	return letter >= 'a' && letter <= 'z'
}
