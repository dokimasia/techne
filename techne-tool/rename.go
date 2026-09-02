// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// RenameInput is what an agent sends to the rename.symbol tool.
type RenameInput struct {
	Scope    string `json:"scope"              jsonschema:"file or directory, workspace-relative"`
	Name     string `json:"name"               jsonschema:"the declaration to rename, qualified as the language writes it"`
	NewName  string `json:"new_name"           jsonschema:"what to call it"`
	Kind     string `json:"kind,omitempty"     jsonschema:"narrows an ambiguous name to one kind"`
	Language string `json:"language,omitempty" jsonschema:"language to assume"`
	DryRun   *bool  `json:"dry_run,omitempty"  jsonschema:"preview without writing; true when omitted"`
}

// Rename builds the tool that renames a declaration and everything
// referring to it.
func Rename(reads Outliner, writes Writer) (Tool, error) {
	return New(string(edit.RenameSymbol), renameDescription,
		func(ctx context.Context, in RenameInput) (Written, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Written{}, err
			}
			held := writing(scope, in.Language)
			if in.NewName == "" {
				return declined(edit.RenameSymbol, held, in.Name, trust.Refused.String(),
					"there is nothing to rename it to: new_name is what to call it"), nil
			}

			found, target, failure := addressing(ctx, reads, scope, in.Language, in.Name, in.Kind)
			if failure != nil {
				return declined(edit.RenameSymbol, held, in.Name, failure.Code, failure.Reason), nil
			}
			held.Language = string(found.Language)

			return asked(ctx, writes, edit.RenameSymbol, held, in.Name, edit.Request{
				Operation: edit.RenameSymbol,
				Scope:     scope,
				Language:  found.Language,
				Target:    target,
				Args:      edit.Args{edit.ArgNewName: in.NewName},
				DryRun:    previewing(in.DryRun),
			})
		})
}

const renameDescription = "PREFER OVER find-and-replace for renaming a declaration. " +
	"Moves every reference with it, and is refused unless the evidence behind it supports " +
	"the claim that there are no others: a rename that updates nine of ten references " +
	"leaves code that compiles and fails at run time. Previews by default."
