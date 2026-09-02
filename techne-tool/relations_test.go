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

func TestRelations(t *testing.T) {
	t.Parallel()

	t.Run("Relations", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, relating(t).Description(), "PREFER OVER ",
				"an agent reaches for grep unless told why not to")
		})

		t.Run("answers with the far end of each edge", func(t *testing.T) {
			t.Parallel()
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"called-by"}`)
			assert.False(t, got.Failed(), "a declaration the scope holds is answered about")
			assert.Length(t, got.Items, 1, "the engine found one edge")
			assert.Equal(t, got.Items[0].Name, "Caller", "the declaration at the far end is named")
			assert.Equal(t, got.Items[0].Kind, sema.KindFunction, "and its kind is a word")
		})

		t.Run("states the declaration asked about once", func(t *testing.T) {
			t.Parallel()
			// Carrying the near end on each of five hundred callers
			// states one fact five hundred times.
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"called-by"}`)
			assert.Equal(t, got.Of, "Store", "the question is stated once, at the top")

			encoded, err := json.Marshal(got.Items[0])
			assert.NoError(t, err, "an edge is JSON a caller can read")
			for _, field := range []string{`"of"`, `"from"`} {
				assert.NotContains(t, string(encoded), field,
					"and carries no near end of its own to repeat it with")
			}
		})

		t.Run("carries the line the edge was written on", func(t *testing.T) {
			t.Parallel()
			// Fetching each call site is a turn per caller.
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"called-by"}`)
			assert.Equal(t, got.Items[0].Via, "	value := Store()",
				"a caller asking who calls this reads the call")
			assert.Equal(t, got.Items[0].Line, 12,
				"at the line it was written on, counting from one")
		})

		t.Run("answers in the direction that was asked for", func(t *testing.T) {
			t.Parallel()
			// Whichever way an engine stores an edge, the answer runs
			// the way the question did.
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"calls"}`)
			assert.Equal(t, got.Relation, "calls", "the direction is the caller's word for it")
		})

		t.Run("caps the edges when asked", func(t *testing.T) {
			t.Parallel()
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"called-by","limit":0}`)
			assert.Length(t, got.Items, 1, "no cap returns what was found")
		})

		t.Run("refuses a direction nobody has, and lists the ones there are", func(t *testing.T) {
			t.Parallel()
			// A caller told only that its word was wrong tries another
			// guess. One told the ten that exist corrects itself.
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"sideways"}`)
			assert.True(t, got.Failed(), "no relation is called sideways")
			for _, kind := range []string{"calls", "called-by", "implements"} {
				assert.Contains(t, got.Error.Reason, kind, "the directions that exist are named")
			}
		})

		t.Run("says a language is not served rather than answering emptily", func(t *testing.T) {
			t.Parallel()
			got := related(t, `{"scope":"notes.md","name":"Store","relation":"calls"}`)
			assert.True(t, got.Failed(), "nothing outlines a markdown file")
			assert.Equal(t, got.Error.Code, "unsupported",
				"a capability gap is something a caller routes around")
		})

		t.Run("never claims a parser proves absence", func(t *testing.T) {
			t.Parallel()
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"called-by"}`)
			assert.False(t, got.Provenance.SupportsNegativeClaim,
				"a name matched across files is coincidence, so no empty answer proves absence")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each edge with the call under it", func(t *testing.T) {
			t.Parallel()
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"called-by"}`)
			assert.ContainsInOrder(t, got.Render(), []string{
				"called-by Store — 1 site", "a.fx:12", "in Caller", "value := Store()", "syntactic",
			}, "a reader gets the site, what it is in, and the call itself")
		})

		t.Run("writes a refusal as the reason", func(t *testing.T) {
			t.Parallel()
			got := related(t, `{"scope":"a.fx","name":"Store","relation":"sideways"}`)
			assert.Contains(t, got.Render(), "refused", "and says so plainly")
		})
	})
}

// relating builds the relations tool over one edge.
func relating(t *testing.T) tool.Tool {
	t.Helper()
	over := addressable()
	over.edges = []sema.Relation{{
		To: sema.Symbol{Name: "Caller", Kind: sema.KindFunction},
		At: source.Span{
			Path:  "a.fx",
			Start: source.Position{Line: 11, Column: 1},
		},
		Via: "\tvalue := Store()",
	}}
	built, err := tool.Relations(over, over)
	assert.NoError(t, err, "the relations tool builds from a read service")
	return built
}

// related runs the relations tool and decodes what came back.
func related(t *testing.T, input string) tool.RelationsOutput {
	t.Helper()
	result, err := relating(t).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.RelationsOutput
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
