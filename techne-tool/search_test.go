// Copyright ThesmOS B.V. 2026
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

		t.Run("returns the name, the kind and the line of each match", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Digest", "one"), function("DigestAll", "two")),
				`{"text":"Digest","scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].Kind, sema.KindFunction, "the kind of Digest")
			assert.Equal(t, got.Items[0].Line, 1, "the line of Digest")
		})

		t.Run("keeps the order of the engine", func(t *testing.T) {
			t.Parallel()
			got := searched(t, serving(function("Second", ""), function("First", "")), `{"text":"","scope":"a.fx"}`)
			assert.Equal(t, names(got.Items), []string{"Second", "First"}, "the matches")
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
			assert.Contains(t, got, "0 matches", "the render")
			assert.Contains(t, got, "nothing found", "the render")
		})
	})
}
