// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// DocumentInput is the input of the document.symbol tool. Doc is the text of the documentation
// without comment markers. The tool writes the markers of the language, at the indentation and
// the place that the language uses.
type DocumentInput struct {
	Scope    string   `json:"scope"              jsonschema:"file or directory of the declaration, relative to the workspace root"`
	Name     string   `json:"name"               jsonschema:"the declaration, qualified as the language writes it when the name is ambiguous"`
	Doc      string   `json:"doc"                jsonschema:"the text of the documentation, without comment markers"`
	Kind     KindWord `json:"kind,omitempty"     jsonschema:"the kind of the declaration, for a name of several kinds"`
	Language string   `json:"language,omitempty" jsonschema:"the language to ask, in place of the languages of the scope"`
	DryRun   *bool    `json:"dry_run,omitempty"  jsonschema:"preview the change without writing it, true when omitted"`
}

// Document returns the tool that writes documentation onto one declaration. It refuses an
// empty Doc before it looks for the declaration.
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
					"doc is empty: doc is the text to write onto the declaration"), nil
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
	"It writes documentation onto one declaration in the form that the documentation tool of " +
	"the language reads: /// for Rust, /** */ for Java, a docstring inside the body for " +
	"Python, and at the indentation of the declaration. Send the text alone, without comment " +
	"markers. It replaces documentation that is already there. It previews the change unless " +
	"dry_run is false, and refuses a change after which the file does not parse."
