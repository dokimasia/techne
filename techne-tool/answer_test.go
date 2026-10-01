// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"fmt"
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

		t.Run("writes text without JSON", func(t *testing.T) {
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

		t.Run("writes the path of a declaration that states one", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items[0].Path = "core/sema/kind.go"
			assert.Contains(t, a.Render(), "\ncore/sema/kind.go:33  type Symbol struct", "the line of Symbol")
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

		t.Run("writes the path of a member only when it differs from the path of its declaration", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items[0].Path, a.Items[0].Members[0].Path = "core/sema/kind.go", "core/sema/kind.go"
			got := a.Render()
			assert.Contains(t, got, "\n     34  Name string", "the line of Name")
			assert.Equal(t, strings.Count(got, "core/sema/kind.go"), 1, "the occurrences of the path of Symbol")
		})

		t.Run("writes the path of a file once before its declarations", func(t *testing.T) {
			t.Parallel()
			a := answered()
			second := a.Items[0]
			second.Name, second.Line, second.Signature, second.Members = "Other", 40, "type Other struct", nil
			third := second
			third.Name, third.Line, third.Path = "Kind", 5, "core/sema/kind.go"
			a.Items[0].Path, second.Path = "core/sema/symbol.go", "core/sema/symbol.go"
			a.Scope.Path, a.Items, a.ByFile = "", []tool.Declaration{a.Items[0], second, third}, true
			assert.ContainsInOrder(t, a.Render(), []string{
				"\ncore/sema/symbol.go\n   33  type Symbol struct", "\n   40  type Other struct",
				"\ncore/sema/kind.go\n    5  type Other struct",
			}, "the render of two files")
			assert.Equal(t, strings.Count(a.Render(), "core/sema/symbol.go"), 1, "the occurrences of symbol.go")
		})

		t.Run("writes the documentation before the source text", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items[0].Snippet = "type Symbol struct {\n\tName string\n}"
			assert.ContainsInOrder(t, a.Render(), []string{"The second line follows.", "type Symbol struct {"},
				"the order of the documentation and the source text")
		})

		t.Run("writes the documentation after the signature", func(t *testing.T) {
			t.Parallel()
			assert.ContainsInOrder(t, answered().Render(), []string{"type Symbol struct", "Symbol is one declaration."},
				"the order of the signature and the documentation")
		})

		t.Run("writes the summary on the line of the name", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items[0].Signature, a.Items[0].Doc, a.Items[0].Summary = "", "", "It is one declaration."
			assert.Contains(
				t,
				a.Render(),
				"\n   33  struct Symbol  1 field  It is one declaration.\n",
				"the line of Symbol",
			)
		})

		tallies := []struct {
			name    string
			give    []sema.Kind
			want    string
			snipped bool
		}{
			{name: "writes the number of the members of each kind", give: []sema.Kind{
				sema.KindMethod, sema.KindField, sema.KindMethod,
			}, want: "\n   33  type Symbol struct  2 methods, 1 field\n"},
			{name: "writes the plural of property", give: []sema.Kind{
				sema.KindProperty, sema.KindProperty,
			}, want: "\n   33  type Symbol struct  2 properties\n"},
			{name: "writes no number of members on a line of source text", give: []sema.Kind{
				sema.KindField,
			}, want: "\n   33  type Symbol struct {\n", snipped: true},
		}
		for _, tt := range tallies {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				a := answered()
				a.Items[0].Doc, a.Items[0].Members = "", nil
				for i, kind := range tt.give {
					a.Items[0].Members = append(
						a.Items[0].Members,
						tool.Declaration{Name: fmt.Sprint("m", i), Kind: kind, Line: 34 + i},
					)
				}
				if tt.snipped {
					a.Items[0].Snippet = "type Symbol struct {\n}"
				}
				assert.Contains(t, a.Render(), tt.want, "the line of Symbol")
			})
		}

		t.Run("writes the unit once for an answer about a unit", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Scope.Path = ""
			assert.HasPrefix(t, a.Render(), "core/sema — go, 2 declarations", "the heading")
		})

		t.Run("ends with the tiers of the evidence", func(t *testing.T) {
			t.Parallel()
			assert.HasSuffix(t, answered().Render(), "\nsyntactic, total coverage.\n", "the evidence of the render")
		})

		t.Run("writes a caveat that is not dynamic", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Provenance.Caveats = append(a.Provenance.Caveats, tool.Caveat{Code: "truncated", Note: "1 of 2 returned"})
			assert.HasSuffix(t, a.Render(), "\nsyntactic, total coverage. 1 of 2 returned.\n",
				"the evidence of the render")
		})

		t.Run("writes the dynamic caveats and the negative claim of an empty answer", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items, a.Provenance.SupportsNegativeClaim = nil, true
			assert.HasSuffix(t, a.Render(),
				"\nsyntactic, total coverage. an empty answer here means there are none. a parser matched text.\n",
				"the evidence of the render")
		})

		t.Run("writes no negative claim for an answer that is not empty", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Provenance.SupportsNegativeClaim = true
			assert.NotContains(t, a.Render(), "an empty answer", "the evidence of the render")
		})

		t.Run("writes nothing declared for an answer without declarations", func(t *testing.T) {
			t.Parallel()
			a := answered()
			a.Items = nil
			assert.Contains(t, a.Render(), "nothing declared", "the render")
		})

		t.Run("writes only the failure of a failed answer", func(t *testing.T) {
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
