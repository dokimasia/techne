// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
)

// holder declares a struct with two fields of type Store.
const holder = "package p\n\ntype Holder struct {\n\tone Store\n\ttwo Store\n}\n"

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("qualifies a method by the type of its receiver", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "use.go"},
				at(t, use, "held.Total"))
			assert.NoError(t, err, "Resolve of held.Total")
			assert.Equal(t, got.Items[0].Name, "Total", "the name of the method")
			assert.Equal(t, got.Items[0].ID, subject("Store.Total", sema.KindMethod), "the ID of the method")
		})

		t.Run("returns the kind of the type that a declaration names", func(t *testing.T) {
			t.Parallel()
			e := serving(t, whole())
			for anchor, kind := range map[string]sema.Kind{
				"type Store":            sema.KindStruct,
				"type Reader":           sema.KindInterface,
				"func helper":           sema.KindFunction,
				"func (s *Store) Total": sema.KindMethod,
			} {
				got, err := e.Resolve(t.Context(), engine.Request{Scope: "store.go"}, at(t, store, anchor))
				assert.NoError(t, err, "Resolve of "+anchor)
				assert.Length(t, got.Items, 1, "the declarations at "+anchor)
				assert.Equal(t, got.Items[0].Kind, kind, "the kind at "+anchor)
			}
		})

		t.Run("returns the signature that the type checker writes", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "store.go"},
				at(t, store, "func helper"))
			assert.NoError(t, err, "Resolve of helper")
			assert.Equal(t, got.Items[0].Signature, "func helper() int", "the signature of helper")
		})

		t.Run("returns exported for a name with a capital", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "store.go"},
				at(t, store, "type Store"))
			assert.NoError(t, err, "Resolve of Store")
			assert.Equal(t, got.Items[0].Visibility, sema.Exported, "the visibility of Store")
		})

		t.Run("returns unexported for a name without a capital", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "store.go"},
				at(t, store, "func helper"))
			assert.NoError(t, err, "Resolve of helper")
			assert.Equal(t, got.Items[0].Visibility, sema.Unexported, "the visibility of helper")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("qualifies a field by the type that declares it", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["fields.go"] = holder
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			var fields []string
			for _, edge := range got.Items {
				if edge.To.Kind == sema.KindField {
					fields = append(fields, edge.To.ID.Name())
				}
			}
			assert.Equal(t, fields, []string{"Holder.one", "Holder.two"}, "the qualified names of the fields")
		})

		t.Run("returns the function that contains a use in its body", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Contains(t, edges(got.Items), "Use", "the function of the use in use.go")
		})
	})
}
