// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// RenameInput is the input of the rename.symbol tool.
type RenameInput struct {
	Scope    string   `json:"scope"              jsonschema:"file or directory of the declaration, relative to the workspace root"`
	Name     string   `json:"name"               jsonschema:"the declaration, qualified as the language writes it when the name is ambiguous"`
	NewName  string   `json:"new_name"           jsonschema:"the new name of the declaration"`
	Kind     KindWord `json:"kind,omitempty"     jsonschema:"the kind of the declaration, for a name of several kinds"`
	Language string   `json:"language,omitempty" jsonschema:"the language to ask, in place of the languages of the scope"`
	DryRun   *bool    `json:"dry_run,omitempty"  jsonschema:"preview the change without writing it, true when omitted"`
}

// Rename returns the tool that renames one declaration and every reference to it. It refuses
// an empty NewName before it looks for the declaration.
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
					"new_name is empty: new_name is the new name of the declaration"), nil
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
	"It rewrites every reference with the declaration, and refuses the rename unless the " +
	"evidence behind it shows that there are no other references: a rename that updates nine " +
	"of ten references leaves code that compiles and fails at run time. It previews the change " +
	"unless dry_run is false."
