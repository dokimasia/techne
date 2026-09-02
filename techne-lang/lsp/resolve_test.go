// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// pointing is the position of Store's own name in [content]: line 2,
// column 5. Every case below resolves from there.
func pointing() source.Position {
	return source.Position{Line: 2, Column: 5}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		// The three shapes a definition arrives in are the reason this
		// package takes the protocol bindings as a dependency rather
		// than decoding by hand. Each of these fails silently against a
		// decoder built for one of the others: it reports that the name
		// does not resolve, which is the answer techne exists to be
		// trusted about.
		for mode, why := range map[string]string{
			"":      "a list of locations, which is what most servers send",
			"one":   "one location, unwrapped, which the specification allows",
			"links": "a list of links, which carries the name's own range separately",
		} {
			t.Run("reads a definition sent as "+why, func(t *testing.T) {
				t.Parallel()
				got, err := serving(t, mode, map[string]string{"a.fake": content}).
					Resolve(t.Context(), engine.Request{Scope: "a.fake"}, pointing())

				assert.NoError(t, err, "resolving a name the server binds succeeds")
				assert.Length(t, got.Items, 1, "the name denotes one declaration")
				assert.Equal(t, got.Items[0].Name, "Store",
					"and it is the declaration the server pointed at")
				assert.Equal(t, got.Items[0].Kind, sema.KindStruct,
					"read out of the file rather than out of the location, which carries no kind")
			})
		}

		t.Run("answers a name that denotes nothing with nothing", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "nowhere", map[string]string{"a.fake": content}).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, pointing())

			assert.NoError(t, err, "a name that resolves to nothing is an answer, not a fault")
			assert.Empty(t, got.Items, "and the answer is nothing")
			assert.False(t, got.Skipped, "the file was read, which is different from not reading one")
		})

		t.Run("says it read nothing where the scope is not one of its files", func(t *testing.T) {
			t.Parallel()
			// A position is in a file. Asked about one this language does
			// not claim, this engine has read nothing and must not lower
			// what the engine beside it is worth.
			got, err := serving(t, "", map[string]string{"notes.md": "# notes\n"}).
				Resolve(t.Context(), engine.Request{Scope: "notes.md"}, pointing())

			assert.NoError(t, err, "a scope with nothing to read is not a fault")
			assert.True(t, got.Skipped, "and the engine says it read nothing")
			assert.Empty(t, got.Items, "having found nothing to find")
		})

		t.Run("says it read nothing where the scope is a directory", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Resolve(t.Context(), engine.Request{Scope: "."}, pointing())

			assert.NoError(t, err, "a scope naming no file is not a fault")
			assert.True(t, got.Skipped,
				"a position means nothing against a directory, so nothing was read")
		})

		t.Run("carries the caveat every resolved answer carries", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, pointing())

			assert.NoError(t, err, "resolving succeeds")
			assert.True(t, carries(got.Caveats, trust.CaveatDynamic),
				"a type checker still sees nothing of what is assembled at run time")
		})
	})
}

// carries reports whether an answer named a caveat.
func carries(held []trust.Caveat, code trust.CaveatCode) bool {
	for _, one := range held {
		if one.Code == code {
			return true
		}
	}
	return false
}
