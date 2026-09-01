// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/query"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// SearchInput is what an agent sends to the search tool.
//
// Text is a name or part of one. Kind narrows to one sort of
// declaration and Private includes what a language keeps inside its
// unit. The rest matches the outline tool.
type SearchInput struct {
	Text      string `json:"text"                         jsonschema:"a name, or part of one"`
	Scope     string `json:"scope,omitempty"              jsonschema:"where to look, workspace-relative"`
	Language  string `json:"language,omitempty"           jsonschema:"language to assume"`
	Kind      string `json:"kind,omitempty"               jsonschema:"function|method|type|interface|constant"`
	Private   bool   `json:"private,omitempty"            jsonschema:"include unexported declarations"`
	Limit     int    `json:"limit,omitempty"              jsonschema:"cap on matches"`
	Detail    string `json:"detail,omitempty"             jsonschema:"summary|standard|full"`
	MaxTokens int    `json:"max_tokens,omitempty"         jsonschema:"estimated answer ceiling"`
	Preferred string `json:"preferred_fidelity,omitempty" jsonschema:"syntactic|indexed|resolved"`
}

// Matches is what the search tool returns.
type Matches struct {
	Answer

	// Ambiguous reports that more than one declaration matched. The
	// candidates are ranked and each carries enough to choose between
	// them, so a caller picks rather than asking again.
	Ambiguous bool `json:"ambiguous,omitempty"`
}

// Search builds the tool that finds a declaration by name.
func Search(s *query.Service) (Tool, error) {
	return New("search", searchDescription,
		func(ctx context.Context, in SearchInput) (Matches, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Matches{}, err
			}

			answered, err := s.Search(ctx,
				engine.Request{
					Scope:     scope,
					Language:  source.Language(in.Language),
					Preferred: fidelity(in.Preferred),
				},
				engine.Query{
					Text:    in.Text,
					Kind:    kindOf(in.Kind),
					Private: in.Private,
					Limit:   in.Limit,
				})
			if err != nil {
				return Matches{}, err
			}

			// One match is what the caller was looking for, so the
			// declaration comes back whole. Making them ask again is a
			// round trip spent confirming what the search already knew.
			detail := Detail(in.Detail)
			if len(answered.Items) == 1 && in.Detail == "" {
				detail = Full
			}

			fitted := Fit(answered, Budget{MaxTokens: in.MaxTokens, Detail: detail})
			return Matches{
				Answer:    render(fitted),
				Ambiguous: len(fitted.Items) > 1,
			}, nil
		})
}

const searchDescription = "PREFER OVER grep for finding where something is declared. " +
	"Matches declaration names rather than every line mentioning them, ranks a literal match " +
	"above a partial one, and returns the whole declaration when exactly one matched so no " +
	"second call is needed. Several matches come back ranked with enough to choose between them."

// kindOf reads the kind a caller asked for, and treats a name it does
// not know as no filter rather than as an error.
func kindOf(name string) sema.Kind {
	for _, k := range []sema.Kind{
		sema.KindModule, sema.KindPackage, sema.KindFile, sema.KindType,
		sema.KindStruct, sema.KindEnum, sema.KindEnumMember, sema.KindInterface,
		sema.KindFunction, sema.KindMethod, sema.KindConstructor,
		sema.KindField, sema.KindVariable, sema.KindConstant,
	} {
		if k.String() == name {
			return k
		}
	}
	return sema.KindUnknown
}
