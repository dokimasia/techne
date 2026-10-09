// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// RenameInput is the input of the rename.symbol tool.
type RenameInput struct {
	DryRun   *bool    `json:"dry_run,omitempty"  jsonschema:"preview only, true by default"`
	Scope    string   `json:"scope"              jsonschema:"file or directory of the declaration, relative to the root"`
	Name     string   `json:"name"               jsonschema:"the declaration, qualified as Type.Method when ambiguous"                alias:"symbol"`
	NewName  string   `json:"new_name"           jsonschema:"the new name"`
	Kind     KindWord `json:"kind,omitempty"     jsonschema:"its kind, such as function, method or struct, when the name has several"`
	Language string   `json:"language,omitempty" jsonschema:"a language to ask instead of the scope's"`
	Line     int      `json:"line,omitempty"     jsonschema:"a line of it, counted from one, when the name has several declarations"`
}

// Rename returns the tool that renames one declaration and every reference to it. It refuses
// an empty NewName before it looks for the declaration, and a NewName that is the name of the
// declaration before it plans.
func Rename(reads Outliner, writes Writer) (Tool, error) {
	return New(string(edit.RenameSymbol), renameDescription,
		func(ctx context.Context, in RenameInput) (Written, error) {
			scope, failure := relative(in.Scope)
			if failure != nil {
				return declined(edit.RenameSymbol, Scope{Language: in.Language}, in.Name,
					failure.Code, failure.Reason), nil
			}
			held := writing(scope, in.Language)
			if in.NewName == "" {
				return declined(edit.RenameSymbol, held, in.Name, trust.Refused.String(),
					"new_name is empty: new_name is the new name of the declaration"), nil
			}

			found, target, failure := addressing(ctx, reads, scope, in.Language, in.Name, in.Kind, in.Line)
			if failure != nil {
				return declined(edit.RenameSymbol, held, in.Name, failure.Code, failure.Reason), nil
			}
			held.Language = string(found.Language)
			if in.NewName == found.Name {
				return declined(edit.RenameSymbol, held, in.Name, trust.Refused.String(),
					fmt.Sprintf("new_name is %s, the name that the declaration has", found.Name)), nil
			}

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
	"It rewrites the declaration and every reference, and refuses the rename unless it found " +
	"every one. It previews the change unless dry_run is false."
