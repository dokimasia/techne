// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"encoding/json"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("Symbol", func(t *testing.T) {
		t.Parallel()

		t.Run("has no parent when zero", func(t *testing.T) {
			t.Parallel()
			var zero sema.Symbol
			assert.Empty(t, string(zero.Parent), "parent")
		})

		t.Run("has KindUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero sema.Symbol
			assert.Equal(t, zero.Kind, sema.KindUnknown, "kind")
		})

		t.Run("encodes every field under a JSON tag", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Symbol{
				ID:          "go:./a#F:function",
				Name:        "F",
				Kind:        sema.KindFunction,
				Language:    "go",
				Span:        source.Span{Path: "a/b.go"},
				Parent:      "go:./a#T:type",
				Visibility:  sema.Exported,
				Modifiers:   []string{"static"},
				Annotations: []sema.Annotation{{Name: "Inject", Text: "@Inject"}},
				Signature:   "func F()",
				Doc:         "F does a thing.",
				Snippet:     "func F() {}",
			})
			assert.NoError(t, err, "marshal")

			var fields map[string]any
			assert.NoError(t, json.Unmarshal(encoded, &fields), "unmarshal")
			assert.Equal(t, slices.Sorted(maps.Keys(fields)), []string{
				"annotations", "doc", "id", "kind", "language", "modifiers",
				"name", "parent", "signature", "snippet", "span", "visibility",
			}, "JSON keys")
		})

		t.Run("encodes an unknown visibility", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Symbol{Name: "F"})
			assert.NoError(t, err, "marshal")
			assert.Contains(t, string(encoded), `"visibility":"unknown"`, "encoding")
		})

		t.Run("omits empty modifiers", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Symbol{Name: "F"})
			assert.NoError(t, err, "marshal")
			assert.NotContains(t, string(encoded), "modifiers", "encoding")
		})

		t.Run("omits empty annotations", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Symbol{Name: "F"})
			assert.NoError(t, err, "marshal")
			assert.NotContains(t, string(encoded), "annotations", "encoding")
		})
	})

	t.Run("Annotation", func(t *testing.T) {
		t.Parallel()

		t.Run("encodes every field under a JSON tag", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Annotation{
				Name: "json",
				Text: `json:"id"`,
				Span: source.Span{Path: "a/b.go"},
			})
			assert.NoError(t, err, "marshal")

			var fields map[string]any
			assert.NoError(t, json.Unmarshal(encoded, &fields), "unmarshal")
			assert.Equal(t, slices.Sorted(maps.Keys(fields)), []string{"name", "span", "text"}, "JSON keys")
		})
	})

	t.Run("Annotated", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true for an attached annotation", func(t *testing.T) {
			t.Parallel()
			s := sema.Symbol{Annotations: []sema.Annotation{
				{Name: "Injectable", Text: "@Injectable({scope: 1})"},
				{Name: "derive", Text: "#[derive(Debug)]"},
			}}
			assert.True(t, s.Annotated("Injectable"), "Injectable")
			assert.True(t, s.Annotated("derive"), "derive")
		})

		t.Run("matches the name and not the text", func(t *testing.T) {
			t.Parallel()
			s := sema.Symbol{Annotations: []sema.Annotation{{Name: "Injectable", Text: "@Injectable"}}}
			assert.False(t, s.Annotated("@Injectable"), "text match")
		})

		t.Run("returns false for a symbol without annotations", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sema.Symbol{Name: "F"}.Annotated("Injectable"), "Injectable")
		})
	})

	t.Run("Modified", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true for an attached modifier", func(t *testing.T) {
			t.Parallel()
			s := sema.Symbol{Modifiers: []string{"public", "static", "final"}}
			assert.True(t, s.Modified("static"), "static")
			assert.True(t, s.Modified("public"), "public")
		})

		t.Run("returns false for a symbol without modifiers", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sema.Symbol{Name: "F"}.Modified("static"), "static")
		})
	})

	t.Run("Containers", func(t *testing.T) {
		t.Parallel()

		t.Run("returns -1 for disjoint symbols", func(t *testing.T) {
			t.Parallel()
			got := sema.Containers([]sema.Symbol{spanning("a.go", 0, 10), spanning("a.go", 20, 30)})
			assert.Equal(t, got, []int{-1, -1}, "containers")
		})

		t.Run("returns the smallest container", func(t *testing.T) {
			t.Parallel()
			got := sema.Containers([]sema.Symbol{
				spanning("a.go", 5, 6), spanning("a.go", 0, 100), spanning("a.go", 2, 50),
			})
			assert.Equal(t, got, []int{2, -1, 1}, "containers")
		})

		t.Run("does not nest equal spans", func(t *testing.T) {
			t.Parallel()
			got := sema.Containers([]sema.Symbol{spanning("a.go", 0, 10), spanning("a.go", 0, 10)})
			assert.Equal(t, got, []int{-1, -1}, "containers")
		})

		t.Run("prefers the first of equal containers", func(t *testing.T) {
			t.Parallel()
			got := sema.Containers([]sema.Symbol{
				spanning("a.go", 0, 10), spanning("a.go", 0, 10), spanning("a.go", 2, 3),
			})
			assert.Equal(t, got, []int{-1, -1, 0}, "containers")
		})

		t.Run("ignores spans in other files", func(t *testing.T) {
			t.Parallel()
			got := sema.Containers([]sema.Symbol{spanning("a.go", 0, 100), spanning("b.go", 5, 6)})
			assert.Equal(t, got, []int{-1, -1}, "containers")
		})

		t.Run("matches a pairwise scan on random trees", func(t *testing.T) {
			t.Parallel()
			for seed := range uint64(20) {
				symbols := forest(seed, 400)
				assert.Equal(t, sema.Containers(symbols), pairwise(symbols), "containers")
			}
		})
	})

	t.Run("Locals", func(t *testing.T) {
		t.Parallel()

		// nested is a file with a type holding a method, a function holding a variable,
		// and a constant holding a struct literal with a field.
		nested := []sema.Symbol{
			kinded(sema.KindStruct, 0, 100), kinded(sema.KindMethod, 10, 90),
			kinded(sema.KindFunction, 200, 300), kinded(sema.KindVariable, 210, 290),
			kinded(sema.KindConstant, 400, 500), kinded(sema.KindStruct, 410, 490),
			kinded(sema.KindField, 420, 480),
		}
		locals := sema.Locals(nested, sema.Containers(nested))

		t.Run("returns false for a declaration at the top level", func(t *testing.T) {
			t.Parallel()
			assert.False(t, locals[0], "the struct")
		})

		t.Run("returns false for a member of a type", func(t *testing.T) {
			t.Parallel()
			assert.False(t, locals[1], "the method of the struct")
		})

		t.Run("returns true for a variable inside a function", func(t *testing.T) {
			t.Parallel()
			assert.True(t, locals[3], "the variable of the function")
		})

		t.Run("returns true for a declaration two levels inside a value", func(t *testing.T) {
			t.Parallel()
			assert.True(t, locals[6], "the field of the struct inside the constant")
		})

		t.Run("returns false for every symbol without a container", func(t *testing.T) {
			t.Parallel()
			alone := []sema.Symbol{kinded(sema.KindFunction, 0, 10), kinded(sema.KindVariable, 20, 30)}
			assert.Equal(t, sema.Locals(alone, sema.Containers(alone)), []bool{false, false}, "locals")
		})
	})
}

// kinded returns a symbol of kind that spans from and to in a.go.
func kinded(kind sema.Kind, from, to int) sema.Symbol {
	s := spanning("a.go", from, to)
	s.Kind = kind
	return s
}

func spanning(path source.Path, from, to int) sema.Symbol {
	return sema.Symbol{Span: source.Span{
		Path:  path,
		Start: source.Position{Offset: from},
		End:   source.Position{Offset: to},
	}}
}

// forest returns about n symbols in shuffled order whose spans form trees
// in two files. It includes equal and zero-width spans. Siblings are at least
// one byte apart, so no two spans partially overlap.
func forest(seed uint64, n int) []sema.Symbol {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	var out []sema.Symbol
	var grow func(path source.Path, from, to, depth int)
	grow = func(path source.Path, from, to, depth int) {
		for at := from; at < to && len(out) < n; {
			start := at + r.IntN(3)
			if start > to {
				return
			}
			end := min(start+r.IntN(40), to)
			out = append(out, spanning(path, start, end))
			if r.IntN(10) == 0 {
				out = append(out, spanning(path, start, end))
			}
			if depth < 6 && end > start && r.IntN(2) == 0 {
				grow(path, start, end, depth+1)
			}
			at = end + 1
		}
	}
	for _, path := range []source.Path{"a.go", "b.go"} {
		grow(path, 0, 4000, 0)
	}
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// pairwise computes the containers by comparing every pair of symbols.
func pairwise(symbols []sema.Symbol) []int {
	width := func(s sema.Symbol) int { return s.Span.End.Offset - s.Span.Start.Offset }
	out := make([]int, len(symbols))
	for i, inner := range symbols {
		out[i] = -1
		for j, outer := range symbols {
			if i == j || outer.Span.Path != inner.Span.Path || width(outer) <= width(inner) ||
				outer.Span.Start.Offset > inner.Span.Start.Offset ||
				outer.Span.End.Offset < inner.Span.End.Offset {
				continue
			}
			if out[i] < 0 || width(outer) < width(symbols[out[i]]) {
				out[i] = j
			}
		}
	}
	return out
}
