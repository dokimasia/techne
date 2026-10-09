// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/tool"
)

// searched runs the search tool over service with input, and decodes the output.
func searched(t *testing.T, service *reads, input string) tool.Matches {
	t.Helper()
	built, err := tool.Search(service)
	assert.NoError(t, err, "the error of Search")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Matches
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestSearch(t *testing.T) {
	t.Parallel()

	t.Run("Search", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Search(serving())
			assert.NoError(t, err, "the error of Search")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns the documentation of one match", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Digest", "Digest hashes a file.")), `{"text":"Digest","scope":"a.fx"}`)
			assert.Length(t, got.Items, 1, "the matches")
			assert.Equal(t, got.Items[0].Doc, "Digest hashes a file.", "the documentation of Digest")
			assert.False(t, got.Ambiguous, "Ambiguous of one match")
		})

		t.Run("returns every match of several as ambiguous", func(t *testing.T) {
			t.Parallel()
			over := serving(function("Digest", "one"), function("DigestAll", "two"))
			got := searched(t, over, `{"text":"Digest","scope":"a.fx"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.True(t, got.Ambiguous, "Ambiguous of two matches")
			assert.Equal(t, names(got.Items), []string{"Digest", "DigestAll"}, "the matches")
		})

		t.Run("returns the kind of each match", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Digest", "one"), function("DigestAll", "two")),
				`{"text":"Digest","scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].Kind, sema.KindFunction, "the kind of Digest")
		})

		t.Run("returns the line of each match counted from one", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Digest", "one"), function("DigestAll", "two")),
				`{"text":"Digest","scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].Line, 1, "the line of Digest")
		})

		t.Run("keeps the order of the engine", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Second", ""), function("First", "")), `{"text":"s","scope":"a.fx"}`)
			assert.Equal(t, names(got.Items), []string{"Second", "First"}, "the matches")
		})

		t.Run("refuses a text of white space before it asks the engine", func(t *testing.T) {
			t.Parallel()
			over := serving(function("Digest", ""))
			got := searched(t, over, `{"text":" ","scope":"a.fx"}`)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Contains(t, got.Error.Reason, "outline tool", "the reason of the failure")
			assert.Empty(t, over.searched, "the queries of the engine")
		})

		t.Run("supports no negative claim at the syntactic tier", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(), `{"text":"Nothing","scope":"a.fx"}`)
			assert.Empty(t, got.Items, "the matches")
			assert.False(t, got.Provenance.SupportsNegativeClaim, "the negative claim of the answer")
		})

		t.Run("returns one match at the level of the input", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Digest", "Digest hashes a file.")),
				`{"text":"Digest","scope":"a.fx","detail":"names"}`)
			assert.Empty(t, got.Items[0].Doc, "the documentation of Digest at names")
		})

		t.Run("asks the engine with the kind of the input", func(t *testing.T) {
			t.Parallel()
			over := serving()
			searched(t, over, `{"text":"x","scope":"a.fx","kind":"enum-member"}`)
			assert.Equal(t, over.searched[0].Kind, sema.KindEnumMember, "the kind of the query")
		})

		t.Run("asks the engine with no binding without include", func(t *testing.T) {
			t.Parallel()
			over := serving()
			searched(t, over, `{"text":"f","scope":"a.fx","private":true,"limit":20}`)
			assert.Equal(t, over.searched[0].Include, engine.Bindings(0), "the bindings of the query")
			assert.Equal(t, over.searched[0].Limit, 20, "the limit of the query")
		})

		t.Run("searches for query without text", func(t *testing.T) {
			t.Parallel()
			over := serving(function("Digest", ""))
			got := searched(t, over, `{"query":"Digest","scope":"a.fx"}`)
			assert.Equal(t, names(got.Items), []string{"Digest"}, "the matches")
			assert.Equal(t, over.searched[0].Text, "Digest", "the text of the query")
		})

		t.Run("searches for text over query", func(t *testing.T) {
			t.Parallel()
			over := serving(function("Digest", ""))
			searched(t, over, `{"text":"Digest","query":"Other","scope":"a.fx"}`)
			assert.Equal(t, over.searched[0].Text, "Digest", "the text of the query")
		})

		t.Run("searches the unexported declarations when no exported one matches", func(t *testing.T) {
			t.Parallel()
			hidden := function("digest", "")
			hidden.Visibility = sema.Unexported
			over := serving(hidden)
			got := searched(t, over, `{"text":"digest","scope":"a.fx"}`)
			assert.Equal(t, names(got.Items), []string{"digest"}, "the matches")
			assert.Length(t, over.searched, 2, "the queries of the engine")
			assert.True(t, over.searched[1].Private, "Private of the second query")
		})

		t.Run("searches once when an exported declaration matches", func(t *testing.T) {
			t.Parallel()
			over := serving(function("Digest", ""))
			searched(t, over, `{"text":"Digest","scope":"a.fx"}`)
			assert.Length(t, over.searched, 1, "the queries of the engine")
		})

		t.Run("searches once for a private input that matches nothing", func(t *testing.T) {
			t.Parallel()
			over := serving()
			searched(t, over, `{"text":"digest","scope":"a.fx","private":true}`)
			assert.Length(t, over.searched, 1, "the queries of the engine")
		})

		t.Run("searches once when no engine serves the scope", func(t *testing.T) {
			t.Parallel()
			over := serving()
			got := searched(t, over, `{"text":"digest","scope":"b.fx"}`)
			assert.Equal(t, got.Error.Code, "unsupported", "the code of the failure")
			assert.Length(t, over.searched, 1, "the queries of the engine")
		})

		tests := []struct {
			name string
			kind sema.Kind
			docs []string
			want string
		}{
			{
				name: "returns one package for the clauses of its files",
				kind: sema.KindPackage,
				docs: []string{"", "Package a keeps stores.", ""},
				want: "Package a keeps stores.",
			},
			{
				name: "returns one module for the declarations of its files",
				kind: sema.KindModule,
				docs: []string{"", "Module a keeps stores."},
				want: "Module a keeps stores.",
			},
			{
				name: "returns the first documented clause of a package",
				kind: sema.KindPackage,
				docs: []string{"Package a keeps stores.", "Package a is documented twice."},
				want: "Package a keeps stores.",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				var clauses []sema.Symbol
				for i, doc := range tt.docs {
					clause := function("a", doc)
					clause.Kind = tt.kind
					clause.ID = sema.NewID(fixture, "a", "a", tt.kind)
					clause.Span.Start.Offset, clause.Span.End.Offset = 10*i, 10*i+5
					clauses = append(clauses, clause)
				}
				got := searched(t, serving(clauses...), `{"text":"a","scope":"a.fx"}`)
				assert.Length(t, got.Items, 1, "the matches")
				assert.Equal(t, got.Items[0].Doc, tt.want, "the documentation of the match")
			})
		}

		t.Run("returns every function of one ID", func(t *testing.T) {
			t.Parallel()
			first, second := function("Get", "one"), function("Get", "two")
			second.Span.Start.Offset, second.Span.End.Offset = 10, 15
			got := searched(t, serving(first, second), `{"text":"Get","scope":"a.fx"}`)
			assert.Length(t, got.Items, 2, "the matches")
		})

		t.Run("refuses a path that leaves the workspace", func(t *testing.T) {
			t.Parallel()
			over := serving()
			got := searched(t, over, `{"text":"f","scope":"../b.fx"}`)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Equal(t, got.Error.Reason, `"../b.fx" leaves the workspace root`, "the reason of the failure")
			assert.Empty(t, over.searched, "the queries of the engine")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("starts with the text of the search", func(t *testing.T) {
			t.Parallel()
			got := tool.Matches{
				Text:  "Digest",
				Items: []tool.Declaration{{Name: "Digest", Kind: sema.KindFunction, Line: 3}},
			}.Render()
			assert.HasPrefix(t, got, `"Digest" — 1 match`, "the render")
		})

		t.Run("writes no match for an empty answer", func(t *testing.T) {
			t.Parallel()
			got := tool.Matches{Text: "x"}.Render()
			assert.That(t, got).
				Contains("0 matches", "the render").
				Contains("nothing found", "the render")
		})
	})
}
