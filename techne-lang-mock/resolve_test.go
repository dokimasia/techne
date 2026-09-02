// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("names what a use refers to, across files", func(t *testing.T) {
			t.Parallel()
			// The whole point of a resolving tier. What a name denotes is
			// usually declared elsewhere, and an engine that looked only
			// where it was pointed would answer nothing while claiming it
			// had covered the scope.
			got, err := built(t).Resolve(t.Context(),
				engine.Request{Scope: "src/client.mock"},
				source.Position{Line: 2, Column: 6})

			assert.NoError(t, err, "resolving a position in a served file succeeds")
			assert.Length(t, got.Items, 1, "the use names one declaration")
			assert.Equal(t, got.Items[0].Name, "Store", "the one in the other file")
			assert.Equal(t, string(got.Items[0].Span.Path), "src/store.mock", "which is where it is")
		})

		t.Run("names a declaration pointed at directly", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Resolve(t.Context(),
				engine.Request{Scope: "src/store.mock"},
				source.Position{Line: 1, Column: 5})
			assert.NoError(t, err, "resolving succeeds")
			assert.Length(t, got.Items, 1, "a declaration denotes itself")
			assert.Equal(t, got.Items[0].Name, "Store", "the one under the cursor")
		})

		t.Run("finds nothing where nothing is named", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Resolve(t.Context(),
				engine.Request{Scope: "src/store.mock"},
				source.Position{Line: 1, Column: 0})
			assert.NoError(t, err, "a position on a keyword is a position")
			assert.Empty(t, got.Items, "and denotes nothing")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("finds every reference, in every file", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				storeID(t), sema.ReferencedBy)

			assert.NoError(t, err, "relating a declaration that exists succeeds")
			assert.Length(t, got.Items, 3, "two uses in its own file and one in the other")
			for _, one := range got.Items {
				assert.NotEmpty(t, one.Via, "each edge carries the line it was written on")
				assert.NotEmpty(t, one.To.Name, "and names what holds it")
			}
		})

		t.Run("names the declaration holding each reference", func(t *testing.T) {
			t.Parallel()
			// A caller asking who refers to this wants to know which
			// declaration does, not only which line.
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				storeID(t), sema.ReferencedBy)
			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, holders(got.Items), []string{"Client", "Get", "New"},
				"each edge is attributed to what encloses it")
		})

		t.Run("answers the other direction from the same edges", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				clientID(t), sema.References)
			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, holders(got.Items), []string{"New", "Store"},
				"what a declaration refers to is the same edges read from the other end")
		})

		t.Run("answers nothing for a direction this language has no edge for", func(t *testing.T) {
			t.Parallel()
			// The caller asked something answerable, and the answer is
			// that there are none.
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				storeID(t), sema.Embeds)
			assert.NoError(t, err, "a direction with no edges is not a refusal")
			assert.Empty(t, got.Items, "and none is the answer")
		})

		t.Run("answers nothing for a declaration nothing declares", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				sema.NewID("mock", "src", "Absent", sema.KindType), sema.ReferencedBy)
			assert.NoError(t, err, "asking about nothing is not a fault")
			assert.Empty(t, got.Items, "and answers with nothing")
		})
	})
}

// storeID is the identity of the container every case relates about.
func storeID(t *testing.T) sema.ID {
	t.Helper()
	return sema.NewID("mock", "src", "Store", sema.KindType)
}

// clientID is the identity of the declaration that refers to two others.
func clientID(t *testing.T) sema.ID {
	t.Helper()
	return sema.NewID("mock", "src", "Client", sema.KindFunction)
}

// holders is what each edge names, in order.
func holders(edges []sema.Relation) []string {
	out := make([]string, 0, len(edges))
	for _, one := range edges {
		out = append(out, one.To.Name)
	}
	return out
}
