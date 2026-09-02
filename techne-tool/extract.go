// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// ExtractInput is what an agent sends to the extract.function tool.
//
// FirstLine and LastLine count from one and are both inclusive, which is
// how an editor reports a selection and how a caller reading a diff
// counts.
//
// There is no receiver to name. Where a function hangs is the language's
// answer and the engine's: a selection inside a class extracts to a
// method on it, and one at the top level extracts to a function. A field
// nothing reads is an operation that silently does something other than
// what was asked.
type ExtractInput struct {
	Path      string `json:"path"               jsonschema:"the file the lines are in, workspace-relative"`
	FirstLine int    `json:"first_line"         jsonschema:"first line to extract, counting from one"`
	LastLine  int    `json:"last_line"          jsonschema:"last line to extract, inclusive"`
	NewName   string `json:"new_name"           jsonschema:"what to call the new function"`
	Language  string `json:"language,omitempty" jsonschema:"language to assume"`
	DryRun    *bool  `json:"dry_run,omitempty"  jsonschema:"preview without writing; true when omitted"`
}

// Extract builds the tool that lifts a run of lines into a function.
func Extract(writes Writer) (Tool, error) {
	return New(string(edit.ExtractFunction), extractDescription,
		func(ctx context.Context, in ExtractInput) (Written, error) {
			path, err := relative(in.Path)
			if err != nil {
				return Written{}, err
			}
			held := writing(path, in.Language)

			if in.NewName == "" {
				return declined(edit.ExtractFunction, held, in.NewName, trust.Refused.String(),
					"there is nothing to call it: new_name is the function's name"), nil
			}
			span, failure := selected(path, in.FirstLine, in.LastLine)
			if failure != nil {
				return declined(edit.ExtractFunction, held, in.NewName,
					failure.Code, failure.Reason), nil
			}

			return asked(ctx, writes, edit.ExtractFunction, held, in.NewName, edit.Request{
				Operation: edit.ExtractFunction,
				Scope:     path,
				Language:  source.Language(in.Language),
				Target:    edit.Target{Kind: edit.TargetSpan, Span: span},
				Args:      edit.Args{edit.ArgNewName: in.NewName},
				DryRun:    previewing(in.DryRun),
			})
		})
}

// selected turns the lines a caller read off an editor into the span the
// vocabulary counts in.
//
// The offsets are left unset. Counting bytes needs the file, which only
// an engine has, and a wrong offset would name different code with no
// sign that it had.
func selected(path source.Path, first, last int) (source.Span, *Failure) {
	switch {
	case first < 1:
		return source.Span{}, &Failure{
			Code:   trust.Refused.String(),
			Reason: fmt.Sprintf("first_line %d: lines count from one", first),
		}
	case last < first:
		return source.Span{}, &Failure{
			Code: trust.Refused.String(),
			Reason: fmt.Sprintf(
				"last_line %d is before first_line %d, so the selection is empty", last, first),
		}
	}
	return source.Span{
		Path:  path,
		Start: source.Position{Line: first - 1},
		End:   source.Position{Line: last - 1},
	}, nil
}

const extractDescription = "PREFER OVER cutting lines out and writing a call by hand. " +
	"Lifts a run of lines into a function, works out what it takes and returns, and " +
	"leaves a call in their place. A selection inside a class becomes a method on it. " +
	"Give the lines as an editor numbers them, counting from one and including the " +
	"last. Previews by default."
