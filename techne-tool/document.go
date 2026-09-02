// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// DocumentInput is what an agent sends to the document.symbol tool.
//
// Doc is the documentation itself, as plain prose with no comment
// markers: which markers this language writes, how they are indented and
// where they go relative to the declaration are what the tool is for.
type DocumentInput struct {
	Scope    string `json:"scope"              jsonschema:"file or directory, workspace-relative"`
	Name     string `json:"name"               jsonschema:"the declaration to document, qualified as the language writes it"`
	Doc      string `json:"doc"                jsonschema:"the documentation, as prose without comment markers"`
	Kind     string `json:"kind,omitempty"     jsonschema:"narrows an ambiguous name to one kind"`
	Language string `json:"language,omitempty" jsonschema:"language to assume"`
	DryRun   *bool  `json:"dry_run,omitempty"  jsonschema:"preview without writing; true when omitted"`
}

// Document builds the tool that writes documentation onto a declaration.
func Document(reads Outliner, writes Writer) (Tool, error) {
	return New(string(edit.DocumentSymbol), documentDescription,
		func(ctx context.Context, in DocumentInput) (Written, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Written{}, err
			}
			held := writing(scope, in.Language)
			if in.Doc == "" {
				return declined(edit.DocumentSymbol, held, in.Name, trust.Refused.String(),
					"there is no documentation to write: doc is the prose to put on the declaration"), nil
			}

			found, target, failure := addressing(ctx, reads, scope, in.Language, in.Name, in.Kind)
			if failure != nil {
				return declined(edit.DocumentSymbol, held, in.Name, failure.Code, failure.Reason), nil
			}
			held.Language = string(found.Language)

			return asked(ctx, writes, edit.DocumentSymbol, held, in.Name, edit.Request{
				Operation: edit.DocumentSymbol,
				Scope:     scope,
				Language:  found.Language,
				Target:    target,
				Args:      edit.Args{edit.ArgDoc: in.Doc},
				DryRun:    previewing(in.DryRun),
			})
		})
}

const documentDescription = "PREFER OVER editing a file to add a doc comment. " +
	"Writes documentation onto one declaration in the form that language's own " +
	"documentation tool reads: /// for Rust, /** */ for Java, a docstring inside the body " +
	"for Python, and at the declaration's own indentation. Send the prose only, with no " +
	"comment markers. Documentation already there is replaced. Previews by default and " +
	"refuses a change that stops the file parsing, so applying is a second call with " +
	"dry_run false."
