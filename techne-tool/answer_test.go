// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/tool"
)

// answered returns an answer about core/sema/symbol.go of one struct with one field.
func answered() tool.Answer {
	return tool.Answer{
		Scope: tool.Scope{Language: "go", Unit: "core/sema", Path: "core/sema/symbol.go"},
		Items: []tool.Declaration{{
			Name: "Symbol", Kind: sema.KindStruct, Line: 33,
			Signature: "type Symbol struct",
			Doc:       "Symbol is one declaration.\nThe second line follows.",
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

		t.Run("writes text and no JSON", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.NotContains(t, got, `"name":`, "the render")
			assert.Contains(t, got, "type Symbol struct", "the render")
		})

		t.Run("writes the path of the scope once", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, strings.Count(answered().Render(), "core/sema/symbol.go"), 1,
				"the occurrences of the path in the render")
		})

		t.Run("writes a member one level under its declaration", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.Contains(t, got, "\n   33  type Symbol struct", "the line of Symbol")
			assert.Contains(t, got, "\n     34  Name string", "the line of Name")
		})

		t.Run("writes every line of the documentation", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.Contains(t, got, "Symbol is one declaration.", "the first line of the documentation")
			assert.Contains(t, got, "The second line follows.", "the second line of the documentation")
		})

		t.Run("writes the source text in place of the signature", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items[0].Snippet = "type Symbol struct {\n\tName string\n}"
			got := a.Render()
			assert.Contains(t, got, "\tName string", "the source text")
			assert.Equal(t, strings.Count(got, "type Symbol struct"), 1, "the occurrences of the signature")
		})

		t.Run("ends with the evidence", func(t *testing.T) {
			t.Parallel()
			got := answered().Render()
			assert.ContainsInOrder(t, got, []string{"syntactic", "total coverage", "a parser matched text"},
				"the evidence of the render")
		})

		t.Run("writes nothing declared for an answer without declarations", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items = nil
			assert.Contains(t, a.Render(), "nothing declared", "the render")
		})

		t.Run("writes the code and the reason of a failure", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Error = &tool.Failure{Code: "unsupported", Reason: "no engine serves rust"}
			assert.Equal(t, a.Render(), "unsupported: no engine serves rust\n", "the render")
		})
	})

	t.Run("Failed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for a served answer", func(t *testing.T) {
			t.Parallel()
			assert.False(t, answered().Failed(), "Failed")
		})

		t.Run("returns true for an answer with an error", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Error = &tool.Failure{Code: "refused", Reason: "no such file"}
			assert.True(t, a.Failed(), "Failed")
		})
	})

	t.Run("Answer", func(t *testing.T) {
		t.Parallel()

		t.Run("encodes no status field", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(answered())
			assert.NoError(t, err, "the encoding of the answer")
			assert.NotContains(t, string(encoded), `"status"`, "the encoding of the answer")
		})

		t.Run("encodes no error field for a served answer", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(answered())
			assert.NoError(t, err, "the encoding of the answer")
			assert.NotContains(t, string(encoded), `"error"`, "the encoding of the answer")
		})
	})
}
