// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// many returns an answer of n functions, each with documentation and source text.
func many(n int) tool.Answer {
	items := make([]tool.Declaration, 0, n)
	for i := range n {
		name := fmt.Sprintf("Symbol%d", i)
		items = append(items, tool.Declaration{
			Name:      name,
			Kind:      sema.KindFunction,
			Line:      i + 1,
			Signature: "func " + name + "() int",
			Doc:       strings.Repeat("documentation that costs a caller context. ", 8),
			Snippet:   "func " + name + "() int {\n" + strings.Repeat("\treturn 0\n", 5) + "}",
		})
	}
	return tool.Answer{
		Scope:      tool.Scope{Language: "fixture", Unit: "pkg", Path: "pkg/a.fx"},
		Items:      items,
		Provenance: tool.Provenance{Engine: "parser", Fidelity: "syntactic", Completeness: "total"},
	}
}

// sourced returns an answer of n functions, each with one line of documentation and 30 lines
// of source text, so the documentation of a function fits where its source text does not.
func sourced(n int) tool.Answer {
	out := many(n)
	for i := range out.Items {
		out.Items[i].Doc = out.Items[i].Name + " returns zero."
		out.Items[i].Snippet = out.Items[i].Signature + " {\n" + strings.Repeat("\treturn 0\n", 30) + "}"
	}
	return out
}

// nested returns an answer of one documented struct with n documented fields.
func nested(n int) tool.Answer {
	members := make([]tool.Declaration, 0, n)
	for i := range n {
		name := fmt.Sprintf("field%d", i)
		members = append(members, tool.Declaration{
			Name: name, Kind: sema.KindField, Line: i + 2,
			Signature: name + " string",
			Doc:       strings.Repeat("what this field is for. ", 6),
		})
	}
	return tool.Answer{
		Scope: tool.Scope{Language: "fixture", Unit: "pkg", Path: "pkg/a.fx"},
		Items: []tool.Declaration{{
			Name: "Store", Kind: sema.KindStruct, Line: 1,
			Signature: "type Store struct", Doc: "Store keeps the items of a shop.", Members: members,
		}},
		Provenance: tool.Provenance{Engine: "parser", Fidelity: "syntactic", Completeness: "total"},
	}
}

// truncation returns the note of the truncation caveat of a, or the empty string.
func truncation(a tool.Answer) string {
	for _, c := range a.Provenance.Caveats {
		if c.Code == string(trust.CaveatTruncated) {
			return c.Note
		}
	}
	return ""
}

// rendered returns the bytes of the render of a without the note of its truncation caveat.
func rendered(a tool.Answer) int {
	out := len(a.Render())
	if note := truncation(a); note != "" {
		out -= len(". " + note)
	}
	return out
}

// documented reports whether a declaration of items has documentation.
func documented(items []tool.Declaration) bool {
	for _, d := range items {
		if d.Doc != "" || documented(d.Members) {
			return true
		}
	}
	return false
}

func TestBudget(t *testing.T) {
	t.Parallel()

	t.Run("Fit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an answer within the budget unchanged", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(2), tool.Budget{MaxTokens: 100000})
			assert.Equal(t, got, many(2), "the answer")
		})

		t.Run("keeps every declaration when their thin forms fit", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(40), tool.Budget{MaxTokens: 900})
			assert.Length(t, got.Items, 40, "the declarations of the answer")
		})

		t.Run("returns an answer within the budget at every ceiling", func(t *testing.T) {
			t.Parallel()
			for ceiling := 20; ceiling <= 3000; ceiling += 20 {
				for _, full := range []tool.Answer{many(12), nested(30)} {
					got := tool.Fit(full, tool.Budget{MaxTokens: ceiling})
					if len(got.Items) == 1 && len(got.Items[0].Members) == 0 {
						continue
					}
					assert.True(t, rendered(got)/3 <= ceiling,
						fmt.Sprintf("the tokens of the render at the ceiling %d", ceiling))
				}
			}
		})

		t.Run("returns documentation only with the source text of its declaration", func(t *testing.T) {
			t.Parallel()
			for ceiling := 20; ceiling <= 3000; ceiling += 20 {
				got := tool.Fit(sourced(8), tool.Budget{MaxTokens: ceiling})
				for _, d := range got.Items {
					assert.False(t, d.Doc != "" && d.Snippet == "",
						fmt.Sprintf("the documentation of %s without source text at the ceiling %d", d.Name, ceiling))
				}
			}
		})

		t.Run("keeps every source text at a budget without room for documentation", func(t *testing.T) {
			t.Parallel()
			found := false
			for ceiling := 20; ceiling <= 3000 && !found; ceiling += 5 {
				got := tool.Fit(many(8), tool.Budget{MaxTokens: ceiling})
				found = len(got.Items) == 8 && !documented(got.Items) && everySnippet(got.Items)
			}
			assert.True(t, found, "a ceiling at which 8 declarations keep their source text and no documentation")
		})

		t.Run("puts back the documentation of the declarations that fit", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(nested(200), tool.Budget{MaxTokens: 200})
			assert.True(t, len(got.Items[0].Members) < 200, "the members of Store within 200 tokens")
			assert.Equal(t, got.Items[0].Doc, "Store keeps the items of a shop.", "the documentation of Store")
		})

		t.Run("counts the declarations without documentation in the caveat", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(40), tool.Budget{MaxTokens: 900})
			assert.Contains(t, truncation(got), "without documentation", "the note of the truncation caveat")
		})

		t.Run("counts the declarations that matched and that it returned", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300})
			assert.True(t, len(got.Items) < 400, "the declarations within 300 tokens")
			assert.HasPrefix(t, truncation(got), fmt.Sprintf("400 matched, %d returned", len(got.Items)),
				"the note of the truncation caveat")
		})

		t.Run("counts the members in the declarations that matched", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(nested(200), tool.Budget{MaxTokens: 40})
			assert.HasPrefix(t, truncation(got), "201 matched", "the note of the truncation caveat")
		})

		t.Run("drops the members of a declaration before the declaration", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(nested(60), tool.Budget{MaxTokens: 60})
			assert.Length(t, got.Items, 1, "the declarations of the answer")
			assert.True(t, len(got.Items[0].Members) < 60, "the members of Store within 60 tokens")
		})

		t.Run("keeps one declaration when none fits", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(50), tool.Budget{MaxTokens: 1})
			assert.Length(t, got.Items, 1, "the declarations within 1 token")
			assert.NotEmpty(t, truncation(got), "the note of the truncation caveat")
		})

		t.Run("keeps one declaration when every declaration is unexported", func(t *testing.T) {
			t.Parallel()
			hidden := many(50)
			for i := range hidden.Items {
				hidden.Items[i].Visibility = sema.Unexported
			}
			got := tool.Fit(hidden, tool.Budget{MaxTokens: 1})
			assert.Length(t, got.Items, 1, "the declarations within 1 token")
		})

		t.Run("keeps one declaration when a kind step drops every declaration", func(t *testing.T) {
			t.Parallel()
			imports := many(50)
			for i := range imports.Items {
				imports.Items[i].Kind = sema.KindImport
			}
			got := tool.Fit(imports, tool.Budget{MaxTokens: 1})
			assert.Length(t, got.Items, 1, "the declarations within 1 token")
		})

		t.Run("returns an empty answer unchanged", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(0), tool.Budget{MaxTokens: 1})
			assert.Empty(t, got.Items, "the declarations of the answer")
			assert.Empty(t, truncation(got), "the note of the truncation caveat")
		})

		t.Run("leaves the provenance of the engine unchanged", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(400), tool.Budget{MaxTokens: 300})
			assert.Equal(t, got.Provenance.Fidelity, "syntactic", "the fidelity")
			assert.Equal(t, got.Provenance.Engine, "parser", "the engine")
		})

		t.Run("fits 35000 declarations within the budget", func(t *testing.T) {
			t.Parallel()
			got := tool.Fit(many(35000), tool.Budget{MaxTokens: tool.DefaultMaxTokens})
			assert.True(t, rendered(got)/3 <= tool.DefaultMaxTokens, "the tokens of the render")
			assert.HasPrefix(t, truncation(got), fmt.Sprintf("35000 matched, %d returned", len(got.Items)),
				"the note of the truncation caveat")
		})
	})

	t.Run("DefaultDetail", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Names for the workspace root", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tool.DefaultDetail("."), tool.Names, "the level of .")
		})

		t.Run("returns Signatures for a file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tool.DefaultDetail("a/b.go"), tool.Signatures, "the level of a/b.go")
		})

		t.Run("returns Names for a name that is an extension alone", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tool.DefaultDetail(".gitignore"), tool.Names, "the level of .gitignore")
		})
	})

	t.Run("Levels", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the levels from the smallest", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tool.Levels(), []tool.Detail{tool.Names, tool.Signatures, tool.Docs, tool.Source},
				"the levels")
		})
	})
}

// everySnippet reports whether every declaration of items has source text.
func everySnippet(items []tool.Declaration) bool {
	for _, d := range items {
		if d.Snippet == "" {
			return false
		}
	}
	return true
}

// BenchmarkFit fits an answer of 35000 functions to the default budget.
func BenchmarkFit(b *testing.B) {
	full := many(35000)
	for b.Loop() {
		tool.Fit(full, tool.Budget{})
	}
}
