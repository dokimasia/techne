// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"

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
	MaxTokens int          `json:"max_tokens,omitempty"         jsonschema:"ceiling of the answer in tokens, 6000 when omitted"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence the caller wants: a weaker answer is degraded, not refused"`
}

// Resolve returns the tool that reports the declarations that the name at a position
// denotes.
func Resolve(reads Resolver) (Tool, error) {
	return New("resolve", resolveDescription,
		func(ctx context.Context, in ResolveInput) (Answer, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Answer{}, err
			}
			at, err := position(in.Line, in.Column)
			if err != nil {
				return Answer{}, err
			}
			detail, byDetail := levelOf(in.Detail, scope)
			include, byInclude := bindingsOf(in.Include)
			preferred, byFidelity := fidelityOf(in.Preferred)
			if failure := first(byDetail, byInclude, byFidelity); failure != nil {
				return failed(about(scope, in.Language, engine.Answer[sema.Symbol]{}), failure), nil
			}

			answered, err := reads.Resolve(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: preferred,
			}, at)
			if err != nil {
				return Answer{}, err
			}

			out := published(answered, about(scope, in.Language, answered), detail, include)
			return Fit(out, Budget{MaxTokens: in.MaxTokens}), nil
		})
}

// position returns the position of a line and a column counted from one, as the vocabulary
// counts them from zero. It leaves the offset unset, because only an engine reads the file
// that the offset needs. It returns an error for a line or a column below one.
func position(line, column int) (source.Position, error) {
	if line < 1 {
		return source.Position{}, fmt.Errorf("tool: line %d: lines count from one", line)
	}
	if column < 1 {
		return source.Position{}, fmt.Errorf("tool: column %d: columns count from one", column)
	}
	return source.Position{Line: line - 1, Column: column - 1}, nil
}

const resolveDescription = "PREFER OVER guessing what a name refers to from the text around it. " +
	"It returns the declaration that the name at one position denotes, with the evidence " +
	"behind it: a type checker that binds the name gives a different answer from a parser that " +
	"matches it, and the answer states which it was. Two declarations mean that the name is " +
	"ambiguous, and the caller chooses."
