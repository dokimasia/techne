// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
)

// subject is the identity of one declaration in [content], read out of
// an outline rather than built here: an identity is a language, a unit,
// a name and a kind, and writing one down by hand tests this package
// against the same memory that wrote it.
func subject(t *testing.T, e *lsp.Engine, name string) sema.ID {
	t.Helper()
	got, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
	assert.NoError(t, err, "the case can find what it asks about")
	one, found := named(got.Items, name)
	assert.True(t, found, "the declaration the case is about is there")
	return one.ID
}

func TestRelate(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("names the declaration each use is written inside", func(t *testing.T) {
			t.Parallel()
			// A location is a file and a range. What a caller reading
			// who-uses-this wants is which declaration holds the use, and
			// working that out is this engine's job rather than the
			// caller's.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "asking who refers to a declaration succeeds")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"},
				"one edge per use, each naming what holds it, in file order")
		})

		t.Run("carries the line each use was written on", func(t *testing.T) {
			t.Parallel()
			// A caller asking who uses this wants to read the use.
			// Fetching each one costs a turn per site.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.HasPrefix(t, got.Items[0].Via, "func (s *Store) Get()",
				"the source line, not just its coordinates")
		})

		t.Run("answers calls from the hierarchy rather than from uses", func(t *testing.T) {
			t.Parallel()
			// A name written in a type is a reference and not a call, so
			// answering calls out of the reference list overstates them.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Get"), sema.CalledBy)

			assert.NoError(t, err, "asking who calls a method succeeds")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the caller the hierarchy named")
		})

		t.Run("answers what a declaration calls", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Get"), sema.Calls)

			assert.NoError(t, err, "asking what a method calls succeeds")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "what the hierarchy named")
			assert.Empty(t, string(got.Items[0].At.Path),
				"and no site, because the outgoing ranges are inside the caller "+
					"rather than at the declaration named")
		})

		t.Run("answers what satisfies a type", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ImplementedBy)

			assert.NoError(t, err, "asking what implements a type succeeds")
			assert.Equal(t, edges(got.Items), []string{"Store"},
				"what the one implementation request named")
		})

		t.Run("answers what a type incorporates", func(t *testing.T) {
			t.Parallel()
			// Every language spells it differently — an anonymous field
			// in Go, extends in Java, with in Scala, include in Ruby —
			// and the type hierarchy is the one request that answers all
			// of them.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.Embeds)

			assert.NoError(t, err, "asking what a type takes from succeeds")
			assert.Equal(t, edges(got.Items), []string{"Store"},
				"the supertype the hierarchy named")
		})

		t.Run("answers what incorporates a type", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.EmbeddedBy)

			assert.NoError(t, err, "asking what takes from a type succeeds")
			assert.Equal(t, edges(got.Items), []string{"After"},
				"the subtype the hierarchy named")
		})

		t.Run("declines the hierarchy where the server has none", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeThin, map[string]string{"a.fake": content})
			_, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.Embeds)

			assert.ErrorIs(t, err, engine.ErrDecline,
				"a request the server did not offer is passed on, not failed")
		})

		t.Run("declines a direction no request answers", func(t *testing.T) {
			t.Parallel()
			// Answering none would be a claim that there are none, and a
			// language that has imports would be reported as having no
			// imports rather than as not having been asked.
			// Imports are written in the source rather than resolved
			// from it, so the parser reads them and this declines. A
			// caller asking gets the parser's answer rather than none.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.Imports)

			assert.ErrorIs(t, err, engine.ErrDecline,
				"a direction with nothing behind it declines, so another engine gets a turn")
		})

		t.Run("reads a use in a file outside the workspace", func(t *testing.T) {
			t.Parallel()
			// A server indexes what its own configuration covers, which
			// for a workspace of several modules is wider than the root
			// techne was pointed at. Such a file is named absolutely, and
			// joining that onto the root builds a name carrying the root
			// twice: an existing file reported as missing.
			e := reaching(t, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "a use outside the workspace is still read")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"},
				"and named to the declaration holding it, as any other use is")
		})

		t.Run("declines where the server says the question does not apply", func(t *testing.T) {
			t.Parallel()
			// A server refuses the call hierarchy for a declaration
			// nothing can call. Answering none would claim nothing calls
			// it; failing would lose the answers another engine may have.
			e := serving(t, modeUncallable, map[string]string{"a.fake": content})
			_, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.CalledBy)

			assert.ErrorIs(t, err, engine.ErrDecline, "the question is passed on, not answered")
			assert.Contains(t, err.Error(), "not a function", "carrying the server's own reason")
		})

		t.Run("says it read nothing where the scope holds none of its files", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"notes.md": "# notes\n"}).
				Relate(t.Context(), engine.Request{Scope: "."}, sema.ID("fake::Store"),
					sema.ReferencedBy)

			assert.NoError(t, err, "a scope with nothing to read is not a fault")
			assert.True(t, got.Skipped, "and the engine says it read nothing")
		})
	})
}

// edges is what an answer pointed at, in order.
func edges(held []sema.Relation) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.To.Name)
	}
	return out
}

// A server that has not analysed a file resolves a definition inside it
// from a syntactic index and answers every other question with nothing.
// metals before it has imported a build does exactly that, reporting no
// implementation of a trait a class two lines below extends — over
// resolved binding and total coverage, which is the claim a caller acts
// on by deleting the trait.
//
// What tells the two apart is whether the server produced a view of the
// file at all, which is what producing diagnostics means.
func TestRelateEvidence(t *testing.T) {
	t.Parallel()

	t.Run("an empty answer from a server with no view of the file", func(t *testing.T) {
		t.Parallel()

		t.Run("supports no claim that there are none", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeUngated, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.Implements)

			assert.NoError(t, err, "a server with no view is not a fault")
			assert.Empty(t, got.Items, "and it had nothing to say")
			assert.Equal(t, got.Completeness, trust.ScopePartial,
				"which is not the same as there being nothing to find")
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, got.Completeness),
				"so nothing may be read out of its silence")
		})

		t.Run("says the server was answering about nothing", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeUngated, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.Implements)

			assert.NoError(t, err, "relating succeeds")
			assert.True(t, carries(got.Caveats, trust.CaveatIndexWarming),
				"the caveat names why the answer is worth nothing")
		})
	})

	t.Run("an empty answer from a server that did analyse the file", func(t *testing.T) {
		t.Parallel()

		t.Run("means there are none", func(t *testing.T) {
			t.Parallel()
			// The other half. A gate that doubted every empty answer
			// would never let a caller conclude anything, which is the
			// same uselessness from the other end.
			e := serving(t, modePushes, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.Implements)

			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"a server that analysed the file and found none has found none")
		})
	})
}
