// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

// subject is the identity of one declaration, built the way every engine
// here builds one: the language, the unit, the bare name and the kind.
func subject(name string, kind sema.Kind) sema.ID {
	return sema.NewID("go", ".", name, kind)
}

func TestRelate(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("names the declaration each use is written inside", func(t *testing.T) {
			t.Parallel()
			// A location is a file and a range. What a caller reading
			// who-uses-this wants is which declaration holds the use.
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "asking who refers to a type succeeds")
			assert.Equal(t, edges(got.Items), []string{"Total", "Sum", "Wrapped", "Use"},
				"one edge per use, each naming what holds it, in file order")
		})

		t.Run("carries the line each use was written on", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.HasPrefix(t, got.Items[0].Via, "func (s *Store) Total()",
				"the source line, not just its coordinates")
		})

		t.Run("answers who calls a function", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("helper", sema.KindFunction), sema.CalledBy)

			assert.NoError(t, err, "asking who calls a function succeeds")
			assert.Equal(t, edges(got.Items), []string{"Total"}, "the one caller there is")
		})

		t.Run("answers what a function calls", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Total", sema.KindMethod), sema.Calls)

			assert.NoError(t, err, "asking what a method calls succeeds")
			assert.Equal(t, edges(got.Items), []string{"helper"}, "what its body calls")
		})

		t.Run("answers what satisfies an interface", func(t *testing.T) {
			t.Parallel()
			// Satisfying an interface in Go is a property of the two
			// types and of nothing written down, so it is computed
			// rather than looked up — including through the pointer,
			// because a method set declared on *Store satisfies what a
			// Store does not.
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Reader", sema.KindInterface), sema.ImplementedBy)

			assert.NoError(t, err, "asking what implements an interface succeeds")
			assert.Equal(t, edges(got.Items), []string{"Store", "Wrapped"},
				"the types whose method sets satisfy it")
		})

		t.Run("answers what a type satisfies", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.Implements)

			assert.NoError(t, err, "asking what a type satisfies succeeds")
			assert.Equal(t, edges(got.Items), []string{"Reader"}, "the interface it answers to")
		})

		t.Run("answers what a type incorporates", func(t *testing.T) {
			t.Parallel()
			// An anonymous field is what Go spells embedding, and it is
			// read from the declaration rather than from the method set:
			// a type whose methods happen to match does not embed it.
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Wrapped", sema.KindStruct), sema.Embeds)

			assert.NoError(t, err, "asking what a type takes from succeeds")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the type written as a bare field")
		})

		t.Run("answers what incorporates a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.EmbeddedBy)

			assert.NoError(t, err, "asking what takes from a type succeeds")
			assert.Equal(t, edges(got.Items), []string{"Wrapped"}, "the type that embeds it")
		})

		t.Run("refuses an identity that names two declarations", func(t *testing.T) {
			t.Parallel()
			// An identity is a language, a unit, a name and a kind, and a
			// package declaring an interface method beside the method
			// implementing it satisfies one twice. The map they are read
			// out of has no order, so picking would answer a different
			// question on different runs.
			held := whole()
			held["twice.go"] = "package p\n\ntype Other interface{ Total() int }\n"
			_, err := serving(t, held).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Total", sema.KindMethod), sema.Calls)

			assert.ErrorIs(t, err, engine.ErrRefuse, "two declarations are not one")
			assert.Contains(t, err.Error(), "twice.go", "and the reason says where they are")
		})

		t.Run("declines a direction a parser reads for nothing", func(t *testing.T) {
			t.Parallel()
			// Imports are written in the source rather than resolved
			// from it. Answering them here would be the same fact at a
			// thousand times the price, and answering none would be a
			// claim the file imports nothing.
			_, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.Imports)

			assert.ErrorIs(t, err, engine.ErrDecline,
				"a direction the parser owns is passed on, not answered")
		})

		t.Run("says it read nothing where no such declaration is there", func(t *testing.T) {
			t.Parallel()
			// An empty answer would be a claim that nothing relates to
			// it, which is a claim about a declaration that is not there.
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Absent", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "asking about a name nothing declares is not a fault")
			assert.True(t, got.Skipped, "and the engine says it found nothing to be about")
		})

		t.Run("narrows to the scope the request named", func(t *testing.T) {
			t.Parallel()
			// Two packages each declaring Store are two declarations, and
			// a request naming one file is about the one in it.
			held := whole()
			held["other/other.go"] = "package other\n\ntype Store struct{}\n"
			got, err := serving(t, held).Relate(t.Context(),
				engine.Request{Scope: "other/other.go"},
				subject("Store", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Empty(t, got.Items, "nothing uses the one in that file")
		})
	})
}

// A type checker over a program with a fault in it binds the names it
// can and guesses at the rest. Every answer it gives is worth what a
// half-bound program is worth.
func TestRelateOverABrokenBuild(t *testing.T) {
	t.Parallel()

	t.Run("an answer over a workspace that does not compile", func(t *testing.T) {
		t.Parallel()

		t.Run("is worth less than the engine usually is", func(t *testing.T) {
			t.Parallel()
			held := whole()
			held["broken.go"] = broken
			got, err := serving(t, held).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "a broken workspace is still answered about")
			assert.Equal(t, got.Lowered, trust.Indexed,
				"names are bound where the checker could bind them")
			assert.True(t, carries(got.Caveats, trust.CaveatBuildBroken), "and the caveat says why")
		})

		t.Run("is worth the engine's own tier where it compiles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, got.Lowered, trust.None, "nothing lowers a whole program's answer")
		})
	})
}
