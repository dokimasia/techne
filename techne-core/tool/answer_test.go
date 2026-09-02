// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/tool"
)

// answered builds an answer over one struct holding one field.
func answered() tool.Answer {
	return tool.Answer{
		Scope: tool.Scope{Language: "go", Unit: "core/sema", Path: "core/sema/symbol.go"},
		Items: []tool.Declaration{{
			Name: "Symbol", Kind: sema.KindStruct, Line: 33,
			Signature: "type Symbol struct",
			Doc:       "Symbol is one declaration.\nThe second line is a read away.",
			Members: tool.Members{{
				Name: "Name", Kind: sema.KindField, Line: 34, Signature: "Name string",
			}},
		}},
		Provenance: tool.Provenance{
			Engine: "treesitter/go", Fidelity: "syntactic", Completeness: "total",
			Caveats: []tool.Caveat{{Code: "dynamic", Note: "a parser matched text"}},
		},
	}
}

func TestAnswer(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes for a reader rather than a parser", func(t *testing.T) {
			t.Parallel()
			// A tool result carries two halves with two readers. Putting
			// the same JSON in both spends the larger cost twice and
			// hands the model the form it reads worst.
			got := answered().Render()
			assert.NotContains(t, got, `"name":`, "the reading half of a result is not JSON")
			assert.Contains(t, got, "type Symbol struct", "a signature is what a reader came for")
		})

		t.Run("states what the answer is about once", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.Contains(t, got, "core/sema/symbol.go",
				"the file is named in the heading rather than on every item")
			assert.Equal(t, strings.Count(got, "core/sema/symbol.go"), 1,
				"a fact true of the whole answer is stated once")
		})

		t.Run("shows what a declaration holds by nesting it", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.Contains(t, got, "\n   33  type Symbol struct",
				"a declaration is written at the depth it sits")
			assert.Contains(t, got, "\n     34  Name string",
				"what it holds is written under it rather than pointed at from it")
		})

		t.Run("carries the whole documentation a level asked for", func(t *testing.T) {
			t.Parallel()
			// The half a model reads must not carry less than the half
			// it does not. A caller that asked for documentation and got
			// one line of it would have to fetch the rest from the
			// structured half it was not reading.
			got := answered().Render()
			assert.Contains(t, got, "Symbol is one declaration.",
				"a level that was asked for is written out")
			assert.Contains(t, got, "The second line is a read away.",
				"a level that was asked for is written out")
		})

		t.Run("writes the source text once, not beside its own signature", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items[0].Snippet = "type Symbol struct {\n\tName string\n}"
			got := a.Render()
			assert.Contains(t, got, "\tName string", "the source level carries what a declaration does")
			assert.Equal(t, strings.Count(got, "type Symbol struct"), 1,
				"the source text opens with the signature, so writing both says it twice")
		})

		t.Run("ends with the evidence behind it", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.Contains(t, got, "syntactic", "a reader is told how the answer was bound")
			assert.Contains(t, got, "total coverage", "and how much of the scope it covered")
			assert.Contains(t, got, "a parser matched text", "and every limit on it")
		})

		t.Run("says an empty answer found nothing", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items = nil
			assert.Contains(t, a.Render(), "nothing declared",
				"a heading with no lines under it reads as a broken answer")
		})

		t.Run("says why a call could not be served", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Error = &tool.Failure{Code: "unsupported", Reason: "no engine serves rust"}
			got := a.Render()
			assert.Contains(t, got, "unsupported", "a caller routes around a capability gap")
			assert.Contains(t, got, "no engine serves rust",
				"a caller told only no cannot tell a gap from a mistake it could correct")
		})
	})

	t.Run("wire form", func(t *testing.T) {
		t.Parallel()

		t.Run("carries no status", func(t *testing.T) {
			t.Parallel()
			// Every value a status could hold either restates a field
			// beside it or is an error, and the two need different
			// shapes rather than one enum spanning both.
			encoded, err := json.Marshal(answered())
			assert.NoError(t, err, "an answer is what every read tool returns")
			assert.NotContains(t, string(encoded), `"status"`,
				"an answer that ran states its evidence, and one that did not carries an error")
		})

		t.Run("carries no error when an engine answered", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(answered())
			assert.NoError(t, err, "an answer is what every read tool returns")
			assert.NotContains(t, string(encoded), `"error"`,
				"the common path pays nothing for the failure shape")
		})
	})

	t.Run("Failed", func(t *testing.T) {
		t.Parallel()

		t.Run("is false for an answer that ran", func(t *testing.T) {
			t.Parallel()
			assert.False(t, answered().Failed(), "an engine answered, whatever it found")
		})

		t.Run("is true for one that could not be served", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Error = &tool.Failure{Code: "refused", Reason: "no such file"}
			assert.True(t, a.Failed(),
				"a request a model can correct reaches it as a failure rather than an empty success")
		})
	})
}
