// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// ResolveInput is the input of the resolve tool. Line and Column count from one, as an editor
// and a compiler report a position.
type ResolveInput struct {
	Scope     string       `json:"scope"                        jsonschema:"the file of the position, relative to the workspace root"`
	Line      int          `json:"line"                         jsonschema:"line, counted from one"`
	Column    int          `json:"column"                       jsonschema:"column in bytes, counted from one"`
	Language  string       `json:"language,omitempty"           jsonschema:"the language to ask, in place of the language of the file"`
	Detail    Detail       `json:"detail,omitempty"             jsonschema:"the fields of each declaration: signatures when omitted"`
	Include   []Include    `json:"include,omitempty"            jsonschema:"bindings to add beside the declarations that the files offer"`
	MaxTokens int          `json:"max_tokens,omitempty"         jsonschema:"ceiling of the answer in tokens, 6000 when omitted or 0"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence the caller wants: a weaker answer is degraded, not refused"`
}

// Resolve returns the tool that reports the declarations that the name at a position
// denotes.
func Resolve(reads Resolver) (Tool, error) {
	return New("resolve", resolveDescription,
		func(ctx context.Context, in ResolveInput) (Answer, error) {
			scope, failure := relative(in.Scope)
			if failure != nil {
				return failed(Scope{Language: in.Language}, failure), nil
			}
			detail, byDetail := levelOf(in.Detail, scope)
			include, byInclude := bindingsOf(in.Include)
			preferred, byFidelity := fidelityOf(in.Preferred)
			if failure = first(byDetail, byInclude, byFidelity); failure != nil {
				return failed(about(scope, in.Language, engine.Answer[sema.Symbol]{}), failure), nil
			}

			answered, err := reads.Resolve(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: preferred,
			}, source.Position{Line: in.Line - 1, Column: in.Column - 1})
			if err != nil {
				return Answer{}, err
			}

			out := published(answered, about(scope, in.Language, answered), Resolved(answered.Items, detail, include))
			return Fit(out, Budget{MaxTokens: in.MaxTokens}), nil
		})
}

const resolveDescription = "PREFER OVER guessing what a name refers to from the text around it. " +
	"It returns the declaration that the name at one position denotes, with the evidence " +
	"behind it: a type checker that binds the name gives a different answer from a parser that " +
	"matches it, and the answer states which it was. Two declarations mean that the name is " +
	"ambiguous, and the caller chooses."
