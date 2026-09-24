// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// SearchInput is the input of the search tool. The fields after Text are the fields of
// [OutlineInput], and the engine applies Kind, Private and Include before Limit.
type SearchInput struct {
	Text      string       `json:"text"                         jsonschema:"a name, or part of one"`
	Scope     string       `json:"scope,omitempty"              jsonschema:"file or directory to search, relative to the workspace root"`
	Language  string       `json:"language,omitempty"           jsonschema:"the language to ask, in place of the languages of the scope"`
	Kind      KindWord     `json:"kind,omitempty"               jsonschema:"keep the declarations of one kind"`
	Private   bool         `json:"private,omitempty"            jsonschema:"keep the declarations that are not visible outside their unit"`
	Limit     int          `json:"limit,omitempty"              jsonschema:"the number of matches to return"`
	Detail    Detail       `json:"detail,omitempty"             jsonschema:"the fields of each declaration: docs for one match when omitted"`
	Include   []Include    `json:"include,omitempty"            jsonschema:"bindings to add beside the declarations that the files offer"`
	Tests     bool         `json:"tests,omitempty"              jsonschema:"read the files that the language treats as tests"`
	MaxTokens int          `json:"max_tokens,omitempty"         jsonschema:"ceiling of the answer in tokens, 6000 when omitted"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence the caller wants: a weaker answer is degraded, not refused"`
}

// Matches is the output of the search tool.
type Matches struct {
	Answer

	// Text is the text of the search.
	Text string `json:"text"`

	// Ambiguous reports that the answer has more than one declaration. The engine ranks
	// them, and each has its name, its kind and its line.
	Ambiguous bool `json:"ambiguous,omitempty"`
}

// Render returns the matches as text, headed by the text of the search and the count of the
// matches.
func (m Matches) Render() string {
	if m.Error != nil {
		return m.Answer.Render()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%q — %s\n\n", m.Text, plural(count(m.Items), "match", "matches"))
	m.body(&b, "nothing found")
	return b.String()
}

// Search returns the tool that finds declarations by name. The answer of one match is at the
// level [Docs] unless the input names a level.
func Search(reads Searcher) (Tool, error) {
	return New("search", searchDescription,
		func(ctx context.Context, in SearchInput) (Matches, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return Matches{}, err
			}
			kind, byKind := kindOf(in.Kind)
			detail, byDetail := levelOf(in.Detail, scope)
			include, byInclude := bindingsOf(in.Include)
			preferred, byFidelity := fidelityOf(in.Preferred)
			if failure := first(byKind, byDetail, byInclude, byFidelity); failure != nil {
				return Matches{
					Answer: failed(about(scope, in.Language, engine.Answer[sema.Symbol]{}), failure),
					Text:   in.Text,
				}, nil
			}

			answered, err := reads.Search(ctx,
				engine.Request{
					Scope:     scope,
					Language:  source.Language(in.Language),
					Preferred: preferred,
					Tests:     in.Tests,
				},
				engine.Query{
					Text:    in.Text,
					Kind:    kind,
					Private: in.Private,
					Include: include,
					Limit:   in.Limit,
				})
			if err != nil {
				return Matches{}, err
			}

			if len(answered.Items) == 1 && in.Detail == DetailUnset {
				detail = Docs
			}
			fitted := Fit(published(answered, about(scope, in.Language, answered), detail, include),
				Budget{MaxTokens: in.MaxTokens})
			return Matches{
				Answer: fitted, Text: in.Text, Ambiguous: len(fitted.Items) > 1,
			}, nil
		})
}

const searchDescription = "PREFER OVER grep for finding where something is declared. " +
	"It matches the names of declarations, not every line that mentions them, and ranks a " +
	"literal match above a partial one. One match comes back whole, with its documentation, " +
	"so no second call is needed. Several matches come back ranked, each with its name, its " +
	"kind and its line."
