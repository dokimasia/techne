// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// MoveInput is what an agent sends to the move.file tool.
type MoveInput struct {
	Path     string `json:"path"               jsonschema:"the file to move, workspace-relative"`
	To       string `json:"to"                 jsonschema:"where it goes, workspace-relative"`
	Language string `json:"language,omitempty" jsonschema:"language to assume"`
	DryRun   *bool  `json:"dry_run,omitempty"  jsonschema:"preview without writing; true when omitted"`
}

// Move builds the tool that moves a file and mends what referred to it.
//
// It needs no declaration looked up: a file names itself, which is why
// this is the one write tool that takes no read service.
func Move(writes Writer) (Tool, error) {
	return New(string(edit.MoveFile), moveDescription,
		func(ctx context.Context, in MoveInput) (Written, error) {
			from, err := relative(in.Path)
			if err != nil {
				return Written{}, err
			}
			to, err := relative(in.To)
			if err != nil {
				return Written{}, err
			}

			held := writing(from, in.Language)
			if in.To == "" {
				return declined(edit.MoveFile, held, string(from), trust.Refused.String(),
					"there is nowhere to move it to: to is the path it takes"), nil
			}
			if from == to {
				return declined(edit.MoveFile, held, string(from), trust.Refused.String(),
					"the file is already there"), nil
			}

			return asked(ctx, writes, edit.MoveFile, held, string(from), edit.Request{
				Operation: edit.MoveFile,
				Scope:     from,
				Language:  source.Language(in.Language),
				Target:    edit.Target{Kind: edit.TargetFile, Path: from},
				Args:      edit.Args{edit.ArgDestination: string(to)},
				DryRun:    previewing(in.DryRun),
			})
		})
}

const moveDescription = "PREFER OVER moving a file and fixing the imports by hand. " +
	"Moves the file and rewrites what referred to it, or refuses when the evidence cannot " +
	"support the claim that every reference was found. Previews by default."
