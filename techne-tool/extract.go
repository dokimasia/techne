// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// ExtractInput is the input of the extract.function tool. FirstLine and LastLine count from
// one, and the selection includes both, as an editor reports a selection. The engine places
// the function: a selection inside a class becomes a method of the class, and one at the top
// level a function.
type ExtractInput struct {
	DryRun    *bool  `json:"dry_run,omitempty"  jsonschema:"preview only, true by default"`
	Path      string `json:"path"               jsonschema:"the file, relative to the root"`
	NewName   string `json:"new_name"           jsonschema:"the name of the new function"`
	Language  string `json:"language,omitempty" jsonschema:"a language to ask instead of the file's"`
	FirstLine int    `json:"first_line"         jsonschema:"the first line, counted from one"`
	LastLine  int    `json:"last_line"          jsonschema:"the last line, included"`
}

// Extract returns the tool that moves a run of lines into a new function and calls the
// function in their place. It refuses a path that [filed] refuses, an empty NewName and a
// selection that [selected] refuses.
func Extract(writes Writer) (Tool, error) {
	return New(string(edit.ExtractFunction), extractDescription,
		func(ctx context.Context, in ExtractInput) (Written, error) {
			path, failure := relative(in.Path)
			if failure != nil {
				return declined(edit.ExtractFunction, Scope{Language: in.Language}, in.NewName,
					failure.Code, failure.Reason), nil
			}
			held := writing(path, in.Language)
			if failure = filed("path", path); failure != nil {
				return declined(edit.ExtractFunction, held, in.NewName, failure.Code, failure.Reason), nil
			}

			if in.NewName == "" {
				return declined(edit.ExtractFunction, held, in.NewName, trust.Refused.String(),
					"new_name is empty: new_name is the name of the new function"), nil
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

// selected returns the span of the lines from first to last of the file at path, counted
// from zero as the vocabulary counts them. It leaves the offsets unset, because only an engine
// reads the file that they need. It refuses a last line before the first.
func selected(path source.Path, first, last int) (source.Span, *Failure) {
	if last < first {
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

const extractDescription = "PREFER OVER cutting lines into a new function by hand. " +
	"It moves the lines from first_line to last_line into a new function, and calls the " +
	"function in their place. It previews the change unless dry_run is false."
