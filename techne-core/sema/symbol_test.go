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
				Doc:        "F does a thing.",
				Snippet:    "func F() {}",
			})
			assert.NoError(t, err, "a symbol is what every read tool returns")

			var wire map[string]any
			assert.NoError(t, json.Unmarshal(encoded, &wire), "a symbol round trips through JSON")

			assert.Equal(t, slices.Sorted(maps.Keys(wire)),
				[]string{
					"doc", "id", "kind", "language", "name",
					"parent", "snippet", "span", "visibility",
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
	})

	t.Run("Doc", func(t *testing.T) {
		t.Parallel()

		t.Run("is not needed to identify the declaration", func(t *testing.T) {
			t.Parallel()
			thinned := sema.Symbol{ID: "go:./a#F:function", Name: "F", Kind: sema.KindFunction}
			assert.NotEmpty(t, string(thinned.ID),
				"the output budget drops Doc first, so identity survives without it")
			assert.NotEmpty(t, thinned.Name,
				"the output budget drops Doc first, so identity survives without it")
			assert.NotEqual(t, thinned.Kind, sema.KindUnknown,
				"the output budget drops Doc first, so identity survives without it")
		})
	})
}
