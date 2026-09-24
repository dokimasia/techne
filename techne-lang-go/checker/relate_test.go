// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
)

// fake is an in-package test file that declares a type with the method of Reader.
const fake = "package p\n\ntype Fake struct{}\n\nfunc (Fake) Sum() int { return 0 }\n"

func TestRelate(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration that contains each use", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, edges(got.Items), []string{"Total", "Sum", "Wrapped", "Use"},
				"the declarations of the uses")
		})

		t.Run("returns the source line of each use", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.HasPrefix(t, got.Items[0].Via, "func (s *Store) Total()", "the source line of the first use")
		})

		t.Run("returns the caller of a function", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("helper", sema.KindFunction), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of helper")
			assert.Equal(t, edges(got.Items), []string{"Total"}, "the callers of helper")
		})

		t.Run("returns the functions that a method calls", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store.Total", sema.KindMethod), sema.Calls)
			assert.NoError(t, err, "Relate of the calls of Total")
			assert.Equal(t, edges(got.Items), []string{"helper"}, "the callees of Total")
		})

		t.Run("returns one callee for two instances of a generic method", func(t *testing.T) {
			t.Parallel()
			generic := "package p\n\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Get() T { return b.v }\n\n" +
				"func Both() int {\n\ta, b := Box[int]{}, Box[string]{}\n\t_ = b.Get()\n\treturn a.Get()\n}\n"
			got, err := serving(t, map[string]string{"box.go": generic}).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Both", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Relate of the calls of Both")
			assert.Equal(t, edges(got.Items), []string{"Get"}, "the callees of Both")
		})

		t.Run("returns the types that satisfy an interface", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Reader", sema.KindInterface), sema.ImplementedBy)
			assert.NoError(t, err, "Relate of the implementations of Reader")
			assert.Equal(t, edges(got.Items), []string{"Store", "Wrapped"}, "the implementations of Reader")
		})

		t.Run("returns a type of an in-package test file that satisfies an interface", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["fake_test.go"] = fake
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Reader", sema.KindInterface), sema.ImplementedBy)
			assert.NoError(t, err, "Relate of the implementations of Reader")
			assert.Equal(t, edges(got.Items), []string{"Fake", "Store", "Wrapped"}, "the implementations of Reader")
		})

		t.Run("returns a type of an importing package that satisfies an interface of a tested package",
			func(t *testing.T) {
				t.Parallel()
				got, err := serving(t, map[string]string{
					"p/p.go":               "package p\n\ntype Item struct{}\n\ntype Getter interface{ Get() Item }\n",
					"p/p_internal_test.go": "package p\n\nimport \"testing\"\n\nfunc TestItem(t *testing.T) {}\n",
					"q/q.go": "package q\n\nimport \"example.com/p/p\"\n\ntype Impl struct{}\n\n" +
						"func (Impl) Get() p.Item { return p.Item{} }\n",
				}).Relate(t.Context(), engine.Request{Scope: "."},
					sema.NewID(golang.Language, "p", "Getter", sema.KindInterface), sema.ImplementedBy)
				assert.NoError(t, err, "Relate of the implementations of Getter")
				assert.Equal(t, edges(got.Items), []string{"Impl"}, "the implementations of Getter")
			})

		t.Run("returns the interfaces that a type satisfies", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.Implements)
			assert.NoError(t, err, "Relate of the interfaces of Store")
			assert.Equal(t, edges(got.Items), []string{"Reader"}, "the interfaces of Store")
		})

		t.Run("returns the types that a type embeds", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Wrapped", sema.KindStruct), sema.Embeds)
			assert.NoError(t, err, "Relate of the embeddings of Wrapped")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the types that Wrapped embeds")
		})

		t.Run("returns the types that embed a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.EmbeddedBy)
			assert.NoError(t, err, "Relate of the types that embed Store")
			assert.Equal(t, edges(got.Items), []string{"Wrapped"}, "the types that embed Store")
		})

		t.Run("matches an ID of another kind by its unit and qualified name", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindType), sema.EmbeddedBy)
			assert.NoError(t, err, "Relate of Store as a type")
			assert.Equal(t, edges(got.Items), []string{"Wrapped"}, "the types that embed Store")
		})

		t.Run("refuses an ID that two declarations have", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["store_test.go"] = "package p_test\n\ntype Store struct{}\n"
			_, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: ".", Tests: true},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrRefuse, "Relate of Store in the package and its external test")
			assert.Contains(t, err.Error(), "store_test.go:3", "the site of the second declaration")
		})

		t.Run("leaves out a declaration in a test file without tests", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["fake_test.go"] = fake
			_, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Fake", sema.KindStruct), sema.Implements)
			assert.ErrorIs(t, err, engine.ErrDecline, "Relate of Fake without tests")
		})

		t.Run("declines an ID that no declaration in the scope has", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Absent", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "Relate of a name that nothing declares")
			assert.Contains(t, err.Error(), "Absent", "the ID in the reason")
		})

		t.Run("returns a skipped result for a scope without a Go file", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["docs/notes.md"] = "# notes\n"
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "docs"},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate in a scope without Go files")
			assert.True(t, got.Skipped, "the answer is skipped")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the skipped answer")
		})

		t.Run("declines the imports", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.Imports)
			assert.ErrorIs(t, err, engine.ErrDecline, "Relate of the imports")
		})

		t.Run("finds the declaration in the scope of the request", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["other/other.go"] = "package other\n\ntype Store struct{}\n"
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "other/other.go"},
				sema.NewID(golang.Language, "other", "Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the Store of other")
			assert.Empty(t, got.Items, "the uses of the Store of other")
		})
	})
}
