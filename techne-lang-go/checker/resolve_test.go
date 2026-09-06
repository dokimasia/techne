// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("answers what the name at a position was bound to", func(t *testing.T) {
			t.Parallel()
			// A use denotes what it was declared as. What makes the
			// answer a binding rather than a name match is that two
			// packages each declaring Store have two objects, and the
			// use resolves to exactly one of them.
			got, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "use.go"}, at(t, use, "held := Store"))

			assert.NoError(t, err, "resolving a use succeeds")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declaration it denotes")
			assert.Equal(t, got.Items[0].Kind, sema.KindStruct, "read from what it is underneath")
			assert.Equal(t, string(got.Items[0].Span.Path), "store.go",
				"and the file the declaration is in, not the file the use is in")
		})

		t.Run("takes a line and a column where a caller has no offset", func(t *testing.T) {
			t.Parallel()
			// Whoever asked is looking at an editor rather than at a byte
			// count, and only something holding the file can turn the two
			// into one.
			offset := at(t, use, "held := Store")
			byLine := source.Position{Line: 3, Column: 10}

			e := serving(t, whole())
			one, err := e.Resolve(t.Context(), engine.Request{Scope: "use.go"}, offset)
			assert.NoError(t, err, "resolving by offset succeeds")
			two, err := e.Resolve(t.Context(), engine.Request{Scope: "use.go"}, byLine)
			assert.NoError(t, err, "resolving by line and column succeeds")

			assert.Equal(t, names(two.Items), names(one.Items),
				"the same position named two ways is the same answer")
		})

		t.Run("answers about the declaration itself as readily", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "store.go"}, at(t, store, "func helper"))

			assert.NoError(t, err, "resolving a declaration succeeds")
			assert.Equal(t, names(got.Items), []string{"helper"}, "which denotes itself")
		})

		t.Run("finds nothing where the position is not on a name", func(t *testing.T) {
			t.Parallel()
			// A caller pointing at whitespace has asked a question with
			// no answer, which is not the same as a broken engine.
			got, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "use.go"}, source.Position{Offset: 0})

			assert.NoError(t, err, "pointing at a keyword is not a fault")
			assert.Empty(t, got.Items, "and nothing is denoted there")
		})

		t.Run("declines a file this language does not read", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "go.mod"}, source.Position{Offset: 0})

			assert.ErrorIs(t, err, engine.ErrDecline, "so another engine gets a turn")
		})

		t.Run("says it read nothing about a file no package holds", func(t *testing.T) {
			t.Parallel()
			// A file the build excludes is one the toolchain does not
			// compile, so nothing bound the names in it. An empty answer
			// would be a claim about code that was never read.
			held := whole()
			held["excluded.go"] = "//go:build ignore\n\npackage p\n\nvar Held = Store{}\n"
			got, err := serving(t, held).Resolve(t.Context(),
				engine.Request{Scope: "excluded.go"}, source.Position{Offset: 40})

			assert.NoError(t, err, "asking about a file no package holds is not a fault")
			assert.True(t, got.Skipped, "and the engine says it read nothing")
		})

		t.Run("carries the caveat every bound answer carries", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "use.go"}, at(t, use, "held := Store"))

			assert.NoError(t, err, "resolving succeeds")
			assert.True(t, carries(got.Caveats, trust.CaveatDynamic),
				"reflection and string-keyed dispatch are invisible to every engine")
		})
	})
}
