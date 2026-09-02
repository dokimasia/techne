// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

func TestOutline(t *testing.T) {
	t.Parallel()

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("finds what a scope declares", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "outlining a scope that exists succeeds")
			assert.Equal(t, names(got.Items),
				[]string{"Client", "Store", "size", "Get", "New"},
				"every declaration, file by file in path order, then as each file writes them")
		})

		t.Run("spans a declaration across what is nested in it", func(t *testing.T) {
			t.Parallel()
			// Whoever renders an outline nests by containment, so a
			// declaration reporting only its own line is one nothing can
			// tell holds anything.
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src/store.mock"})
			assert.NoError(t, err, "outlining succeeds")

			store, found := named(got.Items, "Store")
			assert.True(t, found, "the container is there")
			size, found := named(got.Items, "size")
			assert.True(t, found, "and so is what it holds")
			assert.True(t, store.Span.Start.Offset < size.Span.Start.Offset &&
				store.Span.End.Offset >= size.Span.End.Offset,
				"the container's span covers its member's")
		})

		t.Run("reads the documentation written above a declaration", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src/store.mock"})
			assert.NoError(t, err, "outlining succeeds")
			store, _ := named(got.Items, "Store")
			assert.Equal(t, store.Doc, "Store holds items by name.",
				"the marker is punctuation and the text is what was written")
		})

		t.Run("skips a file the language does not claim", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "notes.md"})
			assert.NoError(t, err, "a directory holding several languages is normal")
			assert.Empty(t, got.Items, "so another language's file yields nothing")
		})

		t.Run("carries the caveat every resolved answer carries", func(t *testing.T) {
			t.Parallel()
			// No static analysis sees a name assembled at run time, and
			// an engine claiming to bind names has to say so.
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "outlining succeeds")

			var carried bool
			for _, c := range got.Caveats {
				carried = carried || c.Code == trust.CaveatDynamic
			}
			assert.True(t, carried, "a caller is told what this tier still cannot see")
		})

		t.Run("answers two identical requests identically", func(t *testing.T) {
			t.Parallel()
			e := built(t)
			first, err := e.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "outlining succeeds")
			second, err := e.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "outlining succeeds")
			assert.Equal(t, second.Items, first.Items,
				"a caller diffing two runs sees only changes somebody made")
		})
	})

	t.Run("Search", func(t *testing.T) {
		t.Parallel()

		t.Run("puts an exact match first", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(),
				engine.Request{Scope: "src"}, engine.Query{Text: "Store"})
			assert.NoError(t, err, "searching succeeds")
			assert.NotEmpty(t, got.Items, "the declaration is found")
			assert.Equal(t, got.Items[0].Name, "Store",
				"an engine returns its own best order, and an exact match is the best")
		})

		t.Run("hides what is not visible outside its unit unless asked", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(),
				engine.Request{Scope: "src"}, engine.Query{Text: "size"})
			assert.NoError(t, err, "searching succeeds")
			assert.Empty(t, got.Items, "an unexported name is not part of what a unit offers")

			private, err := built(t).Search(t.Context(),
				engine.Request{Scope: "src"}, engine.Query{Text: "size", Private: true})
			assert.NoError(t, err, "searching succeeds")
			assert.NotEmpty(t, private.Items, "and a caller that asked for it gets it")
		})

		t.Run("narrows to one kind", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "e", Kind: sema.KindFunction, Private: true})
			assert.NoError(t, err, "searching succeeds")
			for _, one := range got.Items {
				assert.Equal(t, one.Kind, sema.KindFunction, "a kind narrows what a name leaves open")
			}
		})

		t.Run("stops at the cap the caller set", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "e", Private: true, Limit: 1})
			assert.NoError(t, err, "searching succeeds")
			assert.Length(t, got.Items, 1, "a caller that asked for one gets one")
		})

		t.Run("finds nothing for a name nothing declares", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "aNameNoFileWouldDeclare", Private: true})
			assert.NoError(t, err, "finding nothing is an answer, not a fault")
			assert.Empty(t, got.Items, "and this engine invents none")
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"having read the whole scope, so the empty answer means there are none")
		})
	})
}

// names is what an outline found, in order.
func names(items []sema.Symbol) []string {
	out := make([]string, 0, len(items))
	for _, one := range items {
		out = append(out, one.Name)
	}
	return out
}

// named finds one declaration by name.
func named(items []sema.Symbol, name string) (sema.Symbol, bool) {
	for _, one := range items {
		if one.Name == name {
			return one, true
		}
	}
	return sema.Symbol{}, false
}
