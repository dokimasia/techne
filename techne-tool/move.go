// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// MoveInput is the input of the move.file tool.
type MoveInput struct {
	Path     string `json:"path"               jsonschema:"the file to move, relative to the workspace root"`
	To       string `json:"to"                 jsonschema:"the destination, relative to the workspace root"`
	Language string `json:"language,omitempty" jsonschema:"the language to ask, in place of the language of the file"`
	DryRun   *bool  `json:"dry_run,omitempty"  jsonschema:"preview the change without writing it, true when omitted"`
}

// Move returns the tool that moves a file and rewrites the references to it. A file is its
// own target, so the tool needs no read service. It refuses an empty destination and a
// destination that is the file itself.
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
					"to is empty: to is the destination of the file"), nil
			}
			if from == to {
				return declined(edit.MoveFile, held, string(from), trust.Refused.String(),
					"the file is already at "+string(to)), nil
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
	"It moves the file and rewrites the references to it, and refuses the move when the " +
	"evidence does not show that it found every reference. It previews the change unless " +
	"dry_run is false."
