// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("sits at the top level of no unit", func(t *testing.T) {
			t.Parallel()
			var unset sema.Symbol
			assert.Empty(t, string(unset.Parent), "an unset symbol is not nested inside anything")
			assert.Equal(t, unset.Kind, sema.KindUnknown, "an unset symbol claims no kind")
			assert.Equal(t, unset.Visibility, sema.VisibilityUnknown,
				"an unset symbol claims no visibility")
			assert.Empty(t, unset.Modifiers, "an unset symbol carries no keyword")
			assert.Empty(t, unset.Annotations, "an unset symbol carries no metadata")
		})
	})

	t.Run("wire form", func(t *testing.T) {
		t.Parallel()

		t.Run("names every key itself, so no Go field name reaches a caller", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Symbol{
				ID:         "go:./a#F:function",
				Name:       "F",
				Kind:       sema.KindFunction,
				Language:   "go",
				Span:       source.Span{Path: "a/b.go"},
				Parent:     "go:./a#T:type",
				Visibility: sema.Exported,
				Modifiers:  []string{"static"},
				Annotations: []sema.Annotation{
					{Name: "Inject", Text: "@Inject"},
				},
				Doc:     "F does a thing.",
				Snippet: "func F() {}",
			})
			assert.NoError(t, err, "a symbol is what every read tool returns")

			var wire map[string]any
			assert.NoError(t, json.Unmarshal(encoded, &wire), "a symbol round trips through JSON")

			assert.Equal(t, slices.Sorted(maps.Keys(wire)),
				[]string{
					"annotations", "doc", "id", "kind", "language", "modifiers",
					"name", "parent", "snippet", "span", "visibility",
				},
				"the wire form is the contract, so an untagged field is a break in it")
		})

		t.Run("carries visibility even where the engine could not tell", func(t *testing.T) {
			t.Parallel()
			// Absent would read as "not reported". Unknown is a report.
			encoded, err := json.Marshal(sema.Symbol{Name: "F"})
			assert.NoError(t, err, "a symbol is what every read tool returns")
			assert.Contains(t, string(encoded), `"visibility":"unknown"`,
				"a caller filtering to public API must not read unsure as unexported")
		})

		t.Run("leaves out metadata the declaration does not carry", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Symbol{Name: "F"})
			assert.NoError(t, err, "a symbol is what every read tool returns")
			assert.NotContains(t, string(encoded), "modifiers",
				"an empty list costs every symbol in an answer two keys that say nothing")
			assert.NotContains(t, string(encoded), "annotations",
				"an empty list costs every symbol in an answer two keys that say nothing")
		})
	})

	t.Run("Annotated", func(t *testing.T) {
		t.Parallel()

		t.Run("finds an annotation the declaration carries", func(t *testing.T) {
			t.Parallel()
			sym := sema.Symbol{Annotations: []sema.Annotation{
				{Name: "Injectable", Text: "@Injectable({scope: 1})"},
				{Name: "derive", Text: "#[derive(Debug)]"},
			}}
			assert.True(t, sym.Annotated("Injectable"),
				"a tool deciding what to emit asks by name rather than scanning")
			assert.True(t, sym.Annotated("derive"),
				"a tool deciding what to emit asks by name rather than scanning")
		})

		t.Run("matches the name rather than the text", func(t *testing.T) {
			t.Parallel()
			sym := sema.Symbol{Annotations: []sema.Annotation{
				{Name: "Injectable", Text: "@Injectable({scope: 1})"},
			}}
			assert.False(t, sym.Annotated("@Injectable"),
				"the punctuation and the arguments are in Text, and the name is what is matched")
		})

		t.Run("is false where the declaration carries none", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sema.Symbol{Name: "F"}.Annotated("Injectable"),
				"a declaration carrying no metadata carries no particular metadata")
		})
	})

	t.Run("Modified", func(t *testing.T) {
		t.Parallel()

		t.Run("finds a keyword the declaration carries", func(t *testing.T) {
			t.Parallel()
			sym := sema.Symbol{Modifiers: []string{"public", "static", "final"}}
			assert.True(t, sym.Modified("static"),
				"for the languages that spell visibility as a keyword this is where it is read")
			assert.True(t, sym.Modified("public"),
				"for the languages that spell visibility as a keyword this is where it is read")
		})

		t.Run("is false where the declaration carries none", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sema.Symbol{Name: "F"}.Modified("static"),
				"a declaration carrying no keyword carries no particular keyword")
		})
	})
}

func TestAnnotation(t *testing.T) {
	t.Parallel()

	t.Run("wire form", func(t *testing.T) {
		t.Parallel()

		t.Run("names every key itself", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.Annotation{
				Name: "json",
				Text: `json:"id"`,
				Span: source.Span{Path: "a/b.go"},
			})
			assert.NoError(t, err, "an annotation travels with the symbol carrying it")

			var wire map[string]any
			assert.NoError(t, json.Unmarshal(encoded, &wire),
				"an annotation round trips through JSON")
			assert.Equal(t, slices.Sorted(maps.Keys(wire)), []string{"name", "span", "text"},
				"the wire form is the contract, so an untagged field is a break in it")
		})
	})

	t.Run("Span", func(t *testing.T) {
		t.Parallel()

		t.Run("locates the annotation rather than what it is attached to", func(t *testing.T) {
			t.Parallel()
			declaration := source.Span{
				Path:  "a/b.java",
				Start: source.Position{Offset: 0},
				End:   source.Position{Offset: 40},
			}
			a := sema.Annotation{
				Name: "Inject",
				Text: "@Inject",
				Span: source.Span{
					Path:  "a/b.java",
					Start: source.Position{Offset: 0},
					End:   source.Position{Offset: 7},
				},
			}
			assert.True(t, a.Span.End.Offset < declaration.End.Offset,
				"a tool rewriting only the annotation needs the annotation's own bytes")
		})
	})
}
