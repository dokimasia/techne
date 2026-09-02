// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// many builds an answer holding n documented declarations, each
// carrying the source text of its declaration.
func many(n int) tool.Answer {
	items := make([]tool.Declaration, 0, n)
	for i := range n {
		items = append(items, tool.Declaration{
			Name:      "Symbol" + string(rune('A'+i%26)),
			Kind:      sema.KindFunction,
			Line:      i + 1,
			Signature: "func Symbol" + string(rune('A'+i%26)) + "() int",
			Doc:       strings.Repeat("documentation that costs a caller context. ", 8),
			Snippet:   strings.Repeat("func SymbolA() { return }\n", 6),
		})
	}
	return tool.Answer{
		Scope:      tool.Scope{Language: "fixture", Unit: "pkg", Path: "pkg/a.fx"},
		Items:      items,
		Provenance: tool.Provenance{Engine: "parser", Fidelity: "syntactic", Completeness: "total"},
	}
}

// nested builds one declaration holding n members, so the drop order
// can be watched taking leaves before branches.
func nested(n int) tool.Answer {
	members := make([]tool.Declaration, 0, n)
	for i := range n {
		members = append(members, tool.Declaration{
			Name: "field" + string(rune('a'+i%26)), Kind: sema.KindField, Line: i + 2,
			Signature: "field" + string(rune('a'+i%26)) + " string",
			Doc:       strings.Repeat("what this field is for. ", 6),
		})
	}
	return tool.Answer{
		Scope: tool.Scope{Language: "fixture", Unit: "pkg", Path: "pkg/a.fx"},
		Items: []tool.Declaration{{
			Name: "Store", Kind: sema.KindStruct, Line: 1,
			Signature: "type Store struct", Members: members,
		}},
		Provenance: tool.Provenance{Engine: "parser", Fidelity: "syntactic", Completeness: "total"},
	}
}

func truncated(a tool.Answer) bool {
	for _, c := range a.Provenance.Caveats {
		if c.Code == string(trust.CaveatTruncated) {
			return true
		}
	}
	return false
}

func documented(a tool.Answer) bool {
	for _, d := range a.Items {
		if d.Doc != "" {
			return true
		}
	}
	return false
}

func snippeted(a tool.Answer) bool {
	for _, d := range a.Items {
		if d.Snippet != "" {
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
			got := tool.Fit(full, tool.Budget{MaxTokens: 100000})
			assert.Length(t, got.Items, 2, "an answer inside the budget is returned whole")
			assert.False(t, truncated(got), "nothing was dropped, so nothing is claimed to be")
		})

		t.Run("thins every item before dropping any item", func(t *testing.T) {
			t.Parallel()
			// Fifty names with no documentation answers what is there.
			// Eight complete entries answer a different question.
			got := tool.Fit(many(40), tool.Budget{MaxTokens: 900})
			assert.False(t, documented(got), "documentation goes before any item does")
			assert.True(t, len(got.Items) > 8, "thinning bought room for names that would have been dropped")
		})

		t.Run("drops documentation before it drops source text", func(t *testing.T) {
			t.Parallel()
			// The ladder is documentation, then source text, then items.
			// Sweeping the ceiling checks the order holds at every width
			// rather than at one hand-picked number.
			for ceiling := 50; ceiling <= 4000; ceiling += 50 {
				got := tool.Fit(many(8), tool.Budget{MaxTokens: ceiling})
				if !snippeted(got) {
					assert.False(t, documented(got),
						"source text is the later rung, so nothing keeps a doc after losing it")
				}
			}
		})

		t.Run("keeps source text at a budget the documentation alone paid for", func(t *testing.T) {
			t.Parallel()
			// Proves the two are separate rungs. Were they one step, no
			// ceiling would ever drop the documentation and keep the code.
			var found bool
			for ceiling := 50; ceiling <= 4000 && !found; ceiling += 25 {
				got := tool.Fit(many(8), tool.Budget{MaxTokens: ceiling})
				found = !documented(got) && snippeted(got) && len(got.Items) == 8
			}
			assert.True(t, found,
				"dropping the prose is enough at some width, and the code then survives whole")
		})

		t.Run("drops source text before it drops any item", func(t *testing.T) {
			t.Parallel()
			// A name a caller can act on outlives the text of a
			// declaration it was not going to read in full.
			for ceiling := 50; ceiling <= 4000; ceiling += 50 {
				got := tool.Fit(many(8), tool.Budget{MaxTokens: ceiling})
				if len(got.Items) < 8 {
					assert.False(t, snippeted(got),
						"items go last, so nothing is dropped while source text remains")
				}
			}
		})

		t.Run("drops items only once thinning is not enough", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300})
			assert.True(t, len(got.Items) < 400, "an answer that cannot fit loses items")
			assert.True(t, truncated(got), "a caller is told items were dropped")
		})

		t.Run("says how many matched against how many came back", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300})
			var note string
			for _, c := range got.Provenance.Caveats {
				if c.Code == string(trust.CaveatTruncated) {
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
			got := tool.Fit(many(50), tool.Budget{MaxTokens: 1})
			assert.NotEmpty(t, got.Items, "an answer that found something returns something")
			assert.True(t, truncated(got), "the rest is accounted for in a caveat")
		})

		t.Run("keeps an empty answer empty", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(0), tool.Budget{MaxTokens: 1})
			assert.Empty(t, got.Items, "nothing matched, so nothing is invented to return")
			assert.False(t, truncated(got), "nothing was dropped from an answer holding nothing")
		})

		t.Run("leaves the provenance the engine earned", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300})
			assert.Equal(t, got.Provenance.Fidelity, "syntactic",
				"thinning an answer does not change the evidence behind it")
			assert.Equal(t, got.Provenance.Engine, "parser",
				"thinning an answer does not change which engine produced it")
		})
	})

	t.Run("nesting", func(t *testing.T) {
		t.Parallel()

		t.Run("drops what a declaration holds before the declaration", func(t *testing.T) {
			t.Parallel()
			// A branch is worth more than its leaves: a caller told a
			// struct exists can ask for its fields, and one told nothing
			// cannot ask at all.
			got := tool.Fit(nested(60), tool.Budget{MaxTokens: 60})
			assert.Length(t, got.Items, 1, "the declaration outlives its members")
			assert.True(t, len(got.Items[0].Members) < 60, "the members are what paid for it")
		})

		t.Run("counts members in what matched", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(nested(200), tool.Budget{MaxTokens: 40})
			var note string
			for _, c := range got.Provenance.Caveats {
				if c.Code == string(trust.CaveatTruncated) {
					note = c.Note
				}
			}
			assert.Contains(t, note, "201 matched",
				"a caller reads how much of the answer it is holding, members included")
		})
	})
}
