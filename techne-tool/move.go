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
	Path     string `json:"path"               jsonschema:"the file to move, relative to the root"`
	To       string `json:"to"                 jsonschema:"its destination, relative to the root"`
	Language string `json:"language,omitempty" jsonschema:"a language to ask instead of the file's"`
	DryRun   *bool  `json:"dry_run,omitempty"  jsonschema:"preview only, true by default"`
}

// Move returns the tool that moves a file and rewrites the references to it. A file is its
// own target, so the tool needs no read service. It refuses a path that [filed] refuses, an
// empty destination and a destination that is the file itself.
func Move(writes Writer) (Tool, error) {
	return New(string(edit.MoveFile), moveDescription,
		func(ctx context.Context, in MoveInput) (Written, error) {
			from, failure := relative(in.Path)
			if failure != nil {
				return declined(edit.MoveFile, Scope{Language: in.Language}, in.Path, failure.Code, failure.Reason), nil
			}
			held := writing(from, in.Language)
			to, failure := relative(in.To)
			if failure = first(failure, filed("path", from)); failure != nil {
				return declined(edit.MoveFile, held, string(from), failure.Code, failure.Reason), nil
			}

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
	"It moves the file and rewrites its references, and refuses the move unless it found " +
	"every one. It previews the change unless dry_run is false."
