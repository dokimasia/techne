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
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// calling returns [addressable] with one relation: a call of Store in the function Caller on
// the zero-based line 11 of a.fx.
func calling() *reads {
	over := addressable()
	over.edges = []sema.Relation{{
		To:  sema.Symbol{Name: "Caller", Kind: sema.KindFunction},
		At:  source.Span{Path: "a.fx", Start: source.Position{Line: 11, Column: 1}},
		Via: "\tvalue := Store()",
	}}
	return over
}

// calls returns [addressable] with n relations, one per line of a.fx.
func calls(n int) *reads {
	over := addressable()
	for i := range n {
		over.edges = append(over.edges, sema.Relation{
			To:  sema.Symbol{Name: fmt.Sprintf("Caller%d", i), Kind: sema.KindFunction},
			At:  source.Span{Path: "a.fx", Start: source.Position{Line: i}},
			Via: fmt.Sprintf("\tvalue%d := Store()", i),
		})
	}
	return over
}

// relatedOver runs the relations tool over service with input, and decodes the output.
func relatedOver(t *testing.T, service *reads, input string) tool.RelationsOutput {
	t.Helper()
	built, err := tool.Relations(service, service)
	assert.NoError(t, err, "the error of Relations")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.RelationsOutput
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

// truncations returns the notes of the truncation caveats of p.
func truncations(p tool.Provenance) []string {
	var out []string
	for _, c := range p.Caveats {
		if c.Code == string(trust.CaveatTruncated) {
			out = append(out, c.Note)
		}
	}
	return out
}

func TestRelations(t *testing.T) {
	t.Parallel()

	store := `{"scope":"a.fx","name":"Store","relation":"called-by"}`

	t.Run("Relations", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Relations(calling(), calling())
			assert.NoError(t, err, "the error of Relations")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns the declaration at the far end of each relation", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), store)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Length(t, got.Items, 1, "the relations of Store")
			assert.Equal(t, got.Items[0].Name, "Caller", "the name of the far end")
			assert.Equal(t, got.Items[0].Kind, sema.KindFunction, "the kind of the far end")
		})

		t.Run("states the declaration once at the top", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), store)
			assert.Equal(t, got.Of, "Store", "Of of the output")
			encoded, err := json.Marshal(got.Items[0])
			assert.NoError(t, err, "the encoding of a relation")
			assert.NotContains(t, string(encoded), `"of"`, "the encoding of a relation")
		})

		t.Run("returns the line of each site counted from one", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), store)
			assert.Equal(t, got.Items[0].Via, "\tvalue := Store()", "the source line of the site")
			assert.Equal(t, got.Items[0].Line, 12, "the line of the site")
		})

		t.Run("returns the direction of the input", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), `{"scope":"a.fx","name":"Store","relation":"calls"}`)
			assert.Equal(t, got.Relation, "calls", "the relation of the output")
		})

		t.Run("asks the language of the declaration", func(t *testing.T) {
			t.Parallel()
			over := calling()
			relatedOver(t, over, store)
			assert.Length(t, over.related, 1, "the requests of Relate")
			assert.Equal(t, over.related[0].Language, source.Language("fx"), "the language of the request")
		})

		t.Run("asks with the span of the declaration", func(t *testing.T) {
			t.Parallel()
			over := calling()
			relatedOver(t, over, store)
			assert.Equal(t, over.related[0].Declared, stored()[0].Span, "the declared span of the request")
			assert.Equal(t, over.relating[0], stored()[0].ID, "the ID of the request")
		})

		t.Run("asks with the limit of the input", func(t *testing.T) {
			t.Parallel()
			over := calling()
			relatedOver(t, over, `{"scope":"a.fx","name":"Store","relation":"called-by","limit":7}`)
			assert.Equal(t, over.related[0].Limit, 7, "the limit of the request")
		})

		t.Run("asks with DefaultRelations without a limit", func(t *testing.T) {
			t.Parallel()
			over := calling()
			relatedOver(t, over, store)
			assert.Equal(t, over.related[0].Limit, tool.DefaultRelations, "the limit of the request")
		})

		t.Run("returns the first relations up to the limit with a truncation caveat", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calls(60), store)
			assert.Length(t, got.Items, tool.DefaultRelations, "the relations of Store")
			assert.Equal(t, truncations(got.Provenance), []string{"50 of 60 relations returned"},
				"the notes of the truncation caveats")
		})

		t.Run("returns the first relations that fit the budget with a truncation caveat", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calls(40), `{"scope":"a.fx","name":"Store","relation":"called-by","max_tokens":100}`)
			assert.True(t, len(got.Items) < 40, "the relations of Store within 100 tokens")
			note := fmt.Sprintf("%d of 40 relations returned within the token budget", len(got.Items))
			assert.Equal(t, truncations(got.Provenance), []string{note}, "the notes of the truncation caveats")
			assert.True(t, (len(got.Render())-len(". "+note))/3 <= 100, "the tokens of the render without the caveat")
		})

		t.Run("returns one relation under a budget that fits none", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calls(5), `{"scope":"a.fx","name":"Store","relation":"called-by","max_tokens":1}`)
			assert.Length(t, got.Items, 1, "the relations of Store within 1 token")
		})

		t.Run("returns the receiver of a method without a parent as its container", func(t *testing.T) {
			t.Parallel()
			over := calling()
			over.edges[0].To = sema.Symbol{
				ID: sema.NewID("fx", "a", "Store.Get", sema.KindMethod), Name: "Get", Kind: sema.KindMethod,
			}
			got := relatedOver(t, over, store)
			assert.Equal(t, got.Items[0].In, "Store", "the container of the method Get")
		})

		t.Run("returns the qualified name of the parent as the container", func(t *testing.T) {
			t.Parallel()
			over := calling()
			over.edges[0].To = sema.Symbol{
				Name: "size", Kind: sema.KindField, Parent: sema.NewID("fx", "a", "Shop.Store", sema.KindStruct),
			}
			got := relatedOver(t, over, store)
			assert.Equal(t, got.Items[0].In, "Shop.Store", "the container of the field size")
		})

		t.Run("returns an unsupported failure for a scope that no engine serves", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), `{"scope":"notes.md","name":"Store","relation":"calls"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Code, "unsupported", "the code of the failure")
		})

		t.Run("supports no negative claim at the syntactic tier", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), store)
			assert.False(t, got.Provenance.SupportsNegativeClaim, "the negative claim of the answer")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each site with its far end and its source line", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), store)
			assert.ContainsInOrder(t, got.Render(), []string{
				"called-by Store — 1 site", "a.fx:12", "in Caller", "value := Store()", "syntactic",
			}, "the render of the output")
		})

		t.Run("writes the qualified name of a far end with a container", func(t *testing.T) {
			t.Parallel()
			got := tool.RelationsOutput{Relation: "calls", Of: "F", Items: []tool.Connected{
				{Name: "Get", In: "Store", Path: "a.fx", Line: 3},
			}}.Render()
			assert.Contains(t, got, "in Store.Get", "the render of the output")
		})

		t.Run("writes the reason of a refusal", func(t *testing.T) {
			t.Parallel()
			got := relatedOver(t, calling(), `{"scope":"a.fx","name":"Get","relation":"calls"}`)
			assert.True(t, strings.Contains(got.Render(), "refused"), "the render of the refusal")
		})
	})
}
