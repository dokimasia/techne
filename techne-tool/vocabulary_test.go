// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// refusal runs built with input and returns the reason of the refused output. The test fails
// when the output is no refusal.
func refusal(t *testing.T, built tool.Tool, input string) string {
	t.Helper()
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	assert.True(t, result.Failed, "Failed of the result of "+input)
	var out struct {
		Error *tool.Failure `json:"error"`
	}
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	assert.NotNil(t, out.Error, "the failure of "+input)
	assert.Equal(t, out.Error.Code, "refused", "the code of the failure of "+input)
	return out.Error.Reason
}

// words returns the word of each value of values.
func words[T interface{ String() string }](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.String())
	}
	return out
}

func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Includes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every word of include", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, words(tool.Includes()), []string{"import", "parameter", "local", "all"}, "the words")
		})
	})

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses an unknown kind with every kind word", func(t *testing.T) {
			t.Parallel()
			reason := refusal(t, outlining(t), `{"scope":"a.fx","kind":"funtcion"}`)
			assert.Contains(t, reason, `kind does not take "funtcion"`, "the reason")
			for _, word := range words(sema.Kinds()) {
				assert.Contains(t, reason, word, "the reason")
			}
		})

		t.Run("refuses an unknown detail with every level", func(t *testing.T) {
			t.Parallel()
			reason := refusal(t, outlining(t), `{"scope":"a.fx","detail":"summary"}`)
			for _, word := range words(tool.Levels()) {
				assert.Contains(t, reason, word, "the reason")
			}
		})

		t.Run("refuses an unknown include with every include word", func(t *testing.T) {
			t.Parallel()
			reason := refusal(t, outlining(t), `{"scope":"a.fx","include":["imports"]}`)
			for _, word := range words(tool.Includes()) {
				assert.Contains(t, reason, word, "the reason")
			}
		})

		t.Run("refuses an unknown fidelity with every fidelity word", func(t *testing.T) {
			t.Parallel()
			reason := refusal(t, outlining(t), `{"scope":"a.fx","preferred_fidelity":"high"}`)
			for _, word := range words(trust.Fidelities()) {
				assert.Contains(t, reason, word, "the reason")
			}
		})

		t.Run("refuses an unknown relation with every relation word", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Relations(calling(), calling())
			assert.NoError(t, err, "the error of Relations")
			reason := refusal(t, built, `{"scope":"a.fx","name":"Store","relation":"sideways"}`)
			for _, word := range words(sema.RelationKinds()) {
				assert.Contains(t, reason, word, "the reason")
			}
		})

		t.Run("refuses an unknown word in every read tool", func(t *testing.T) {
			t.Parallel()
			search, err := tool.Search(serving())
			assert.NoError(t, err, "the error of Search")
			resolve, err := tool.Resolve(serving())
			assert.NoError(t, err, "the error of Resolve")
			verify, err := tool.Verify(serving())
			assert.NoError(t, err, "the error of Verify")
			for name, call := range map[string]struct {
				built tool.Tool
				input string
			}{
				"search":  {search, `{"text":"x","scope":"a.fx","kind":"funtcion"}`},
				"resolve": {resolve, `{"scope":"a.fx","line":1,"column":1,"detail":"summary"}`},
				"verify":  {verify, `{"scope":"a.fx","preferred_fidelity":"high"}`},
			} {
				assert.NotEmpty(t, refusal(t, call.built, call.input), "the reason of "+name)
			}
		})

		t.Run("refuses an unknown kind in a write tool", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Rename(addressable(), &recorder{})
			assert.NoError(t, err, "the error of Rename")
			reason := refusal(t, built, `{"scope":"a.fx","name":"Store","new_name":"Vault","kind":"klass"}`)
			assert.Contains(t, reason, `kind does not take "klass"`, "the reason")
		})

		t.Run("adds the bindings of every include word", func(t *testing.T) {
			t.Parallel()
			over := serving()
			built, err := tool.Search(over)
			assert.NoError(t, err, "the error of Search")
			_, err = built.Execute(t.Context(),
				json.RawMessage(`{"text":"x","scope":"a.fx","include":["import","parameter","local"]}`))
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, over.searched[0].Include, engine.BindImports|engine.BindParameters|engine.BindLocals,
				"the bindings of the query")
		})
	})
}
