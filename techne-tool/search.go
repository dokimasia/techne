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
	"go.dokimi.dev/techne/core/trust"
)

// SearchInput is the input of the search tool. It takes Text under the name query too. The
// fields after Text are the fields of [OutlineInput], and the engine applies Kind, Private and
// Include before Limit.
type SearchInput struct {
	Text      string       `json:"text"                         jsonschema:"a name, part of one, or words that its documentation contains"                      alias:"query"`
	Scope     string       `json:"scope,omitempty"              jsonschema:"file or directory, relative to the root"`
	Language  string       `json:"language,omitempty"           jsonschema:"a language to ask instead of the scope's"`
	Kind      KindWord     `json:"kind,omitempty"               jsonschema:"keep one kind, such as function, method, struct or interface"`
	Private   bool         `json:"private,omitempty"            jsonschema:"include unexported declarations, which a search without an exported match includes"`
	Limit     int          `json:"limit,omitempty"              jsonschema:"most matches to return, all by default"`
	Detail    Detail       `json:"detail,omitempty"             jsonschema:"docs for one match and names for more by default"`
	Include   []Include    `json:"include,omitempty"            jsonschema:"bindings to add"`
	Tests     bool         `json:"tests,omitempty"              jsonschema:"include test files"`
	MaxTokens int          `json:"max_tokens,omitempty"         jsonschema:"answer ceiling in tokens, 6000 by default"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence wanted; the answer states its own fidelity"`
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

// Search returns the tool that finds declarations by name. A search without Private that
// matches no exported declaration searches again with the unexported ones, because a caller who
// names a declaration wants it wherever it is visible. A package is one match, however many
// files state its clause. The answer of one match is at the level [Docs] unless the input names
// a level.
func Search(reads Searcher) (Tool, error) {
	return New("search", searchDescription,
		func(ctx context.Context, in SearchInput) (Matches, error) {
			text := in.Text
			scope, failure := relative(in.Scope)
			if failure != nil {
				return Matches{Answer: failed(Scope{Language: in.Language}, failure), Text: text}, nil
			}
			kind, byKind := kindOf(in.Kind)
			detail, byDetail := levelOf(in.Detail, scope)
			include, byInclude := bindingsOf(in.Include)
			preferred, byFidelity := fidelityOf(in.Preferred)
			if failure = first(texted(text), byKind, byDetail, byInclude, byFidelity); failure != nil {
				return Matches{
					Answer: failed(about(scope, in.Language, engine.Answer[sema.Symbol]{}), failure),
					Text:   text,
				}, nil
			}

			req := engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: preferred,
				Tests:     in.Tests,
			}
			q := engine.Query{Text: text, Kind: kind, Private: in.Private, Include: include, Limit: in.Limit}
			answered, err := reads.Search(ctx, req, q)
			if err != nil {
				return Matches{}, err
			}
			if !q.Private && len(answered.Items) == 0 && answered.Status.Answered() {
				q.Private = true
				if answered, err = reads.Search(ctx, req, q); err != nil {
					return Matches{}, err
				}
			}
			answered.Items = onePackage(answered.Items)

			if len(answered.Items) == 1 && in.Detail == DetailUnset {
				detail = Docs
			}
			items := Declared(answered.Items, detail, include)
			fitted := Fit(published(answered, about(scope, in.Language, answered), items),
				Budget{MaxTokens: in.MaxTokens})
			return Matches{
				Answer: fitted, Text: text, Ambiguous: len(fitted.Items) > 1,
			}, nil
		})
}

// onePackage returns items with one declaration of each package or module: the first of its
// ID with documentation, or else the first of its ID, at the place of the first. Every file of a
// Go package states its clause, and only one states its documentation.
func onePackage(items []sema.Symbol) []sema.Symbol {
	out := make([]sema.Symbol, 0, len(items))
	at := map[sema.ID]int{}
	for _, s := range items {
		if s.Kind != sema.KindPackage && s.Kind != sema.KindModule {
			out = append(out, s)
			continue
		}
		i, seen := at[s.ID]
		switch {
		case !seen:
			at[s.ID] = len(out)
			out = append(out, s)
		case out[i].Doc == "" && s.Doc != "":
			out[i] = s
		}
	}
	return out
}

// texted returns a refusal for a text of white space only, which matches every name.
func texted(text string) *Failure {
	if strings.TrimSpace(text) != "" {
		return nil
	}
	return &Failure{
		Code:   trust.Refused.String(),
		Reason: "text is empty, and search matches a name or part of one. The outline tool lists every declaration",
	}
}

const searchDescription = "PREFER OVER grep for finding where something is declared. " +
	"It matches the names of declarations, best match first, and a text of several words " +
	"matches the documentation that contains every word. One match comes back with its " +
	"documentation."
