// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/tool"
)

func declaration(name, doc string) sema.Symbol {
	return sema.Symbol{
		ID: sema.ID("fixture:./a#" + name + ":function"), Name: name, Kind: sema.KindFunction,
		Language: fixture, Span: source.Span{Path: "a.fx"}, Visibility: sema.Exported, Doc: doc,
	}
}

func searchTool(t *testing.T, found ...sema.Symbol) tool.Tool {
	t.Helper()
	built, err := tool.Search(serving(found...))
	assert.NoError(t, err, "the search tool builds from a read service")
	return built
}

func matches(t *testing.T, built tool.Tool, input string) map[string]any {
	t.Helper()
	got, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")
	var decoded map[string]any
	assert.NoError(t, json.Unmarshal(got.Payload, &decoded), "the result is JSON a caller can read")
	return decoded
}

func TestSearch(t *testing.T) {
	t.Parallel()

	t.Run("Search", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, searchTool(t).Description(), "PREFER OVER ",
				"an agent reaches for grep unless told why not to")
		})

		t.Run("returns the whole declaration when one matched", func(t *testing.T) {
			t.Parallel()
			// An agent that searched for a name wanted that name.
			// Making it ask again is a round trip spent confirming what
			// the search already knew.
			got := matches(t, searchTool(t, declaration("Digest", "Digest hashes a file.")),
				`{"text":"Digest","scope":"a.fx"}`)
			items := got["items"].([]any)
			assert.Length(t, items, 1, "one declaration matched")
			assert.Equal(t, items[0].(map[string]any)["doc"], "Digest hashes a file.",
				"a single match comes back whole, without a second call")
		})

		t.Run("does not call one match ambiguous", func(t *testing.T) {
			t.Parallel()
			got := matches(t, searchTool(t, declaration("Digest", "")), `{"text":"Digest","scope":"a.fx"}`)
			_, flagged := got["ambiguous"]
			assert.False(t, flagged, "nothing to choose between is not a choice")
		})

		t.Run("returns candidates rather than refusing when several matched", func(t *testing.T) {
			t.Parallel()
			// Refusing with "ambiguous" costs a turn and tells the agent
			// nothing it can act on.
			got := matches(t, searchTool(t,
				declaration("Digest", "one"), declaration("DigestAll", "two")),
				`{"text":"Digest","scope":"a.fx"}`)

			_, failed := got["error"]
			assert.False(t, failed, "several matches is an answer, not a refusal")
			assert.Equal(t, got["ambiguous"], true, "the caller is told it has a choice to make")
			assert.Length(t, got["items"].([]any), 2, "every candidate comes back")
		})

		t.Run("gives each candidate enough to choose between them", func(t *testing.T) {
			t.Parallel()
			got := matches(t, searchTool(t,
				declaration("Digest", "one"), declaration("DigestAll", "two")),
				`{"text":"Digest","scope":"a.fx"}`)
			first := got["items"].([]any)[0].(map[string]any)
			assert.NotEmpty(t, first["name"], "a candidate carries the name to pick by")
			assert.NotEmpty(t, first["kind"], "a candidate says what sort of declaration it is")
			assert.NotNil(t, first["line"], "a candidate says where it lives")
		})

		t.Run("keeps the order the engine chose", func(t *testing.T) {
			t.Parallel()
			// A language server ranks with more to go on than a parser.
			// Re-ranking here would throw that away.
			got := matches(t, searchTool(t,
				declaration("Second", ""), declaration("First", "")),
				`{"text":"","scope":"a.fx"}`)
			items := got["items"].([]any)
			assert.Equal(t, items[0].(map[string]any)["name"].(string), "Second",
				"the engine's own order reaches the caller unchanged")
		})

		t.Run("says nothing was found rather than proving absence", func(t *testing.T) {
			t.Parallel()
			got := matches(t, searchTool(t), `{"text":"Nothing","scope":"a.fx"}`)
			assert.Length(t, got["items"].([]any), 0, "the engine matched nothing")
			provenance := got["provenance"].(map[string]any)
			assert.Equal(t, provenance["supportsNegativeClaim"], false,
				"a parser's empty search means none were found, never that there are none")
		})

		t.Run("respects a detail the caller named", func(t *testing.T) {
			t.Parallel()
			// Expanding a single match is a default, not an override.
			got := matches(t, searchTool(t, declaration("Digest", "Digest hashes a file.")),
				`{"text":"Digest","scope":"a.fx","detail":"summary"}`)
			first := got["items"].([]any)[0].(map[string]any)
			_, documented := first["doc"]
			assert.False(t, documented, "a caller that asked for summary gets summary")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("heads the answer with the question, not the place", func(t *testing.T) {
			t.Parallel()
			// A search is asked in words and an outline is asked about a
			// place, so an answer read on its own says which it was.
			got := tool.Matches{
				Text:       "Digest",
				Items:      []tool.Declaration{{Name: "Digest", Kind: sema.KindFunction, Line: 3}},
				Provenance: tool.Provenance{Fidelity: "syntactic", Completeness: "total"},
			}.Render()
			assert.HasPrefix(t, got, `"Digest" — 1 match`,
				"an answer carries the question that produced it")
		})

		t.Run("counts one match without reading as a fault", func(t *testing.T) {
			t.Parallel()
			one := tool.Matches{
				Text:       "x",
				Items:      []tool.Declaration{{Name: "x", Kind: sema.KindVariable, Line: 1}},
				Provenance: tool.Provenance{Fidelity: "syntactic", Completeness: "total"},
			}.Render()
			assert.Contains(t, one, "1 match", "one of a thing is not one things")

			none := tool.Matches{
				Text:       "x",
				Provenance: tool.Provenance{Fidelity: "syntactic", Completeness: "total"},
			}.Render()
			assert.Contains(t, none, "0 matches", "none of a thing is plural")
			assert.Contains(t, none, "nothing found",
				"a heading with no lines under it reads as a broken answer")
		})
	})

	t.Run("scope", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the workspace root as a directory, not a file", func(t *testing.T) {
			t.Parallel()
			// path.Ext reads "." as an extension of ".", so the root was
			// taken for a file: answered at a file's level, and with its
			// path left off every item because the scope named one.
			assert.Equal(t, tool.DefaultDetail("."), tool.Names,
				"the whole workspace is answered at the level a directory is")
			assert.Equal(t, tool.DefaultDetail("a/b.go"), tool.Signatures,
				"a file is answered at the level that replaces reading it")
			assert.Equal(t, tool.DefaultDetail(".gitignore"), tool.Names,
				"a name that is all suffix is not a file with an extension")
		})
	})
}
