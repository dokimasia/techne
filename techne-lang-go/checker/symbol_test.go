// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
)

// Two engines answering about one declaration have to be talking about
// the same thing, so an identity built here has to match one built by a
// parser or a language server over the same code.
func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("a declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the bare name a method is called by", func(t *testing.T) {
			t.Parallel()
			// gopls reports (*Store).Total and a parser reports Total.
			// The identity that has to match both is the one without the
			// receiver.
			got, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "use.go"}, at(t, use, "held.Total"))

			assert.NoError(t, err, "resolving a method succeeds")
			assert.Equal(t, got.Items[0].Name, "Total", "the name it is called by")
			assert.Equal(t, got.Items[0].ID, sema.NewID("go", ".", "Total", sema.KindMethod),
				"and an identity a parser would build the same way")
		})

		t.Run("is the kind it is underneath rather than how it is written", func(t *testing.T) {
			t.Parallel()
			// A named type is whatever it names: a struct, an interface,
			// or a type of its own. Reported as "type" they would all
			// collapse onto one kind and a caller narrowing by it would
			// get every one of them.
			e := serving(t, whole())
			for anchor, kind := range map[string]sema.Kind{
				"type Store":            sema.KindStruct,
				"type Reader":           sema.KindInterface,
				"func helper":           sema.KindFunction,
				"func (s *Store) Total": sema.KindMethod,
			} {
				got, err := e.Resolve(t.Context(),
					engine.Request{Scope: "store.go"}, at(t, store, anchor))
				assert.NoError(t, err, "resolving "+anchor+" succeeds")
				assert.Length(t, got.Items, 1, "one declaration is denoted at "+anchor)
				assert.Equal(t, got.Items[0].Kind, kind, "the kind "+anchor+" is underneath")
			}
		})

		t.Run("carries the signature the type checker bound", func(t *testing.T) {
			t.Parallel()
			// The rendering that agrees with what the compiler bound,
			// which one built from the syntax would not for a type
			// written in another package.
			got, err := serving(t, whole()).Resolve(t.Context(),
				engine.Request{Scope: "store.go"}, at(t, store, "func helper"))

			assert.NoError(t, err, "resolving succeeds")
			assert.Contains(t, got.Items[0].Signature, "func helper() int",
				"what it takes and what it returns")
		})

		t.Run("is visible or not as the language decides", func(t *testing.T) {
			t.Parallel()
			e := serving(t, whole())
			exported, err := e.Resolve(t.Context(),
				engine.Request{Scope: "store.go"}, at(t, store, "type Store"))
			assert.NoError(t, err, "resolving succeeds")
			unexported, err := e.Resolve(t.Context(),
				engine.Request{Scope: "store.go"}, at(t, store, "func helper"))
			assert.NoError(t, err, "resolving succeeds")

			assert.Equal(t, exported.Items[0].Visibility, sema.Exported,
				"a leading capital is what Go spells exported")
			assert.Equal(t, unexported.Items[0].Visibility, sema.Unexported,
				"and the absence of one is what it spells unexported")
		})
	})

	t.Run("the declaration a use is written inside", func(t *testing.T) {
		t.Parallel()

		t.Run("is the field where the use is in one", func(t *testing.T) {
			t.Parallel()
			// Told only the type, a caller reading twenty uses of it sees
			// several collapse onto the same name and cannot tell which
			// member each was. Compared against gopls over one module,
			// this was the only thing the two answers disagreed about.
			held := whole()
			held["fields.go"] = "package p\n\ntype Holder struct {\n\tone Store\n\ttwo Store\n}\n"
			got, err := serving(t, held).Relate(t.Context(),
				engine.Request{Scope: "fields.go"},
				sema.NewID("go", ".", "Holder", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Empty(t, got.Items, "nothing uses Holder")

			uses, err := serving(t, held).Relate(t.Context(), engine.Request{Scope: "."},
				sema.NewID("go", ".", "Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "relating succeeds")
			assert.Contains(t, edges(uses.Items), "one", "the field the use is written in")
			assert.Contains(t, edges(uses.Items), "two", "and the other one, told apart")
		})

		t.Run("is the function where the use is in its body", func(t *testing.T) {
			t.Parallel()
			// A local variable is not somewhere a caller navigates to.
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				sema.NewID("go", ".", "Store", sema.KindStruct), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Contains(t, edges(got.Items), "Use",
				"the function holding the local, rather than the local")
		})
	})
}
