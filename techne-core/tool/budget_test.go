// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/tool"
	"go.dokimi.dev/techne/core/trust"
)

// many builds an answer holding n documented symbols.
func many(n int) engine.Answer[sema.Symbol] {
	items := make([]sema.Symbol, 0, n)
	for i := range n {
		items = append(items, sema.Symbol{
			ID:         sema.ID("fixture:./pkg#Sym:function"),
			Name:       "Symbol" + string(rune('A'+i%26)),
			Kind:       sema.KindFunction,
			Language:   source.Language("fixture"),
			Span:       source.Span{Path: "pkg/a.fx"},
			Visibility: sema.Exported,
			Doc:        strings.Repeat("documentation that costs a caller context. ", 8),
		})
	}
	return engine.Answer[sema.Symbol]{
		Items:      items,
		Status:     trust.OK,
		Provenance: trust.Provenance{Engine: "parser", Fidelity: trust.Syntactic, Completeness: trust.ScopeTotal},
	}
}

func truncated(a engine.Answer[sema.Symbol]) bool {
	for _, c := range a.Provenance.Caveats {
		if c.Code == trust.CaveatTruncated {
			return true
		}
	}
	return false
}

func documented(a engine.Answer[sema.Symbol]) bool {
	for _, s := range a.Items {
		if s.Doc != "" {
			return true
		}
	}
	return false
}

func TestBudget(t *testing.T) {
	t.Parallel()

	t.Run("Fit", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves an answer that fits alone", func(t *testing.T) {
			t.Parallel()
			full := many(2)
			got := tool.Fit(full, tool.Budget{MaxTokens: 100000, Detail: tool.Full})
			assert.Length(t, got.Items, 2, "an answer inside the budget is returned whole")
			assert.False(t, truncated(got), "nothing was dropped, so nothing is claimed to be")
		})

		t.Run("thins every item before dropping any item", func(t *testing.T) {
			t.Parallel()
			// Fifty names with no documentation answers what is there.
			// Eight complete entries answer a different question.
			got := tool.Fit(many(40), tool.Budget{MaxTokens: 900, Detail: tool.Full})
			assert.False(t, documented(got), "documentation goes before any item does")
			assert.True(t, len(got.Items) > 8, "thinning bought room for names that would have been dropped")
		})

		t.Run("drops items only once thinning is not enough", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300, Detail: tool.Full})
			assert.True(t, len(got.Items) < 400, "an answer that cannot fit loses items")
			assert.True(t, truncated(got), "a caller is told items were dropped")
		})

		t.Run("says how many matched against how many came back", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300, Detail: tool.Full})
			var note string
			for _, c := range got.Provenance.Caveats {
				if c.Code == trust.CaveatTruncated {
					note = c.Note
				}
			}
			assert.Contains(t, note, "400",
				"eight entries that do not say how many more exist answer the wrong question")
		})

		t.Run("keeps one item when anything matched", func(t *testing.T) {
			t.Parallel()
			// Zero items with a count reads like an answer nothing
			// served. One item plus a count does not.
			got := tool.Fit(many(50), tool.Budget{MaxTokens: 1, Detail: tool.Full})
			assert.NotEmpty(t, got.Items, "an answer that found something returns something")
			assert.True(t, truncated(got), "the rest is accounted for in a caveat")
		})

		t.Run("keeps an empty answer empty", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(0), tool.Budget{MaxTokens: 1, Detail: tool.Full})
			assert.Empty(t, got.Items, "nothing matched, so nothing is invented to return")
			assert.False(t, truncated(got), "nothing was dropped from an answer holding nothing")
		})

		t.Run("leaves the provenance the engine earned", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300, Detail: tool.Full})
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic,
				"thinning an answer does not change the evidence behind it")
			assert.Equal(t, got.Provenance.Engine, "parser",
				"thinning an answer does not change which engine produced it")
		})
	})

	t.Run("Detail", func(t *testing.T) {
		t.Parallel()

		t.Run("summary carries what identifies a declaration and no more", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(1), tool.Budget{MaxTokens: 100000, Detail: tool.Summary})
			assert.NotEmpty(t, string(got.Items[0].ID), "a summary still identifies the declaration")
			assert.NotEmpty(t, got.Items[0].Name, "a summary still names the declaration")
			assert.Empty(t, got.Items[0].Doc, "a summary carries no documentation")
			assert.NotEmpty(t, string(got.Items[0].Span.Path), "a summary still says which file holds it")
			assert.Equal(t, got.Items[0].Span.Start, source.Position{},
				"the offsets are what standard adds, so a summary carries none")
			assert.Equal(t, got.Items[0].Visibility, sema.VisibilityUnknown,
				"visibility is what standard adds, so a summary claims none")
		})

		t.Run("standard adds where it is without adding what it says", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(1), tool.Budget{MaxTokens: 100000, Detail: tool.Standard})
			assert.NotEmpty(t, string(got.Items[0].Span.Path), "standard says which file declares it")
			assert.Equal(t, got.Items[0].Visibility, sema.Exported, "standard carries the visibility")
			assert.Empty(t, got.Items[0].Doc, "standard carries no documentation")
		})

		t.Run("full carries the documentation", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(1), tool.Budget{MaxTokens: 100000, Detail: tool.Full})
			assert.NotEmpty(t, got.Items[0].Doc, "full is the level that carries documentation")
		})

		t.Run("an unset detail is standard", func(t *testing.T) {
			t.Parallel()
			// An agent that says nothing gets the level that answers
			// most questions without paying for prose.
			got := tool.Fit(many(1), tool.Budget{MaxTokens: 100000})
			assert.NotEmpty(t, string(got.Items[0].Span.Path), "the default says where the declaration is")
			assert.Empty(t, got.Items[0].Doc, "the default carries no documentation")
		})
	})
}
