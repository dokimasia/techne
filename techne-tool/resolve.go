// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// ResolveInput is what an agent sends to the resolve tool.
//
// Line and Column count from one, as an editor reports them and as every
// other answer here prints them. A caller pasting a position out of a
// stack trace or a compiler message should not have to subtract one.
type ResolveInput struct {
	Scope     string   `json:"scope"                        jsonschema:"the file the position is in"`
	Line      int      `json:"line"                         jsonschema:"line, counting from one"`
	Column    int      `json:"column"                       jsonschema:"column in bytes, counting from one"`
	Language  string   `json:"language,omitempty"           jsonschema:"language to assume"`
	Detail    string   `json:"detail,omitempty"             jsonschema:"names|signatures|docs|source"`
	Include   []string `json:"include,omitempty"            jsonschema:"import|parameter|local|all"`
	MaxTokens int      `json:"max_tokens,omitempty"         jsonschema:"estimated answer ceiling"`
	Preferred string   `json:"preferred_fidelity,omitempty" jsonschema:"syntactic|indexed|resolved"`
}

// Resolve builds the tool that reports what a name denotes.
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

			answered, err := reads.Resolve(ctx, engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: fidelity(in.Preferred),
			}, at)
			if err != nil {
				return Answer{}, err
			}

			out := published(answered, about(scope, in.Language, answered),
				level(in.Detail, scope), in.Include)
			return Fit(out, Budget{MaxTokens: in.MaxTokens}), nil
		})
}

// position turns the coordinates a caller reads off an editor into the
// ones the vocabulary counts in.
//
// The offset is left unset. Counting bytes needs the file, which only an
// engine has, and a wrong offset would name a different place with no
// sign that it had.
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
	"Reports which declaration the name at one position denotes, with the evidence behind " +
	"it: a type checker binding the name is a different answer from a parser matching it, " +
	"and the answer says which it was. Two items mean the name is ambiguous and the caller " +
	"chooses."
