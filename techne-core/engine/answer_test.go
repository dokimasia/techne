// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
)

func TestAnswer(t *testing.T) {
	t.Parallel()

	t.Run("Answer", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no payload when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Answer[sema.Symbol]
			assert.False(t, zero.Status.Answered(), "Answered")
		})

		t.Run("supports no negative claim when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Answer[sema.Symbol]
			assert.False(t, zero.Provenance.SupportsNegativeClaim(), "SupportsNegativeClaim")
		})
	})

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		t.Run("sets Kind to KindUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Query
			assert.Equal(t, zero.Kind, sema.KindUnknown, "Kind")
		})

		t.Run("includes no binding when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Query
			assert.Equal(t, zero.Include, engine.Bindings(0), "Include")
		})
	})

	t.Run("Keeps", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps a declaration that a file offers", func(t *testing.T) {
			t.Parallel()
			for _, kind := range []sema.Kind{sema.KindFunction, sema.KindStruct, sema.KindField} {
				assert.True(t, engine.Bindings(0).Keeps(kind, false), kind.String())
			}
		})

		t.Run("leaves out every binding when zero", func(t *testing.T) {
			t.Parallel()
			none := engine.Bindings(0)
			assert.False(t, none.Keeps(sema.KindImport, false), "an import")
			assert.False(t, none.Keeps(sema.KindParameter, false), "a parameter")
			assert.False(t, none.Keeps(sema.KindTypeParameter, false), "a type parameter")
			assert.False(t, none.Keeps(sema.KindLabel, false), "a label")
			assert.False(t, none.Keeps(sema.KindVariable, true), "a local variable")
		})

		t.Run("keeps each binding under its own flag", func(t *testing.T) {
			t.Parallel()
			assert.True(t, engine.BindImports.Keeps(sema.KindImport, false), "an import")
			assert.True(t, engine.BindParameters.Keeps(sema.KindParameter, true), "a parameter")
			assert.True(t, engine.BindParameters.Keeps(sema.KindTypeParameter, true), "a type parameter")
			assert.True(t, engine.BindLocals.Keeps(sema.KindVariable, true), "a local variable")
			assert.True(t, engine.BindLabels.Keeps(sema.KindLabel, true), "a label")
		})

		t.Run("reads the kind of a binding before its place", func(t *testing.T) {
			t.Parallel()
			assert.False(t, engine.BindLocals.Keeps(sema.KindParameter, true), "a parameter inside a function")
			assert.False(t, engine.BindLocals.Keeps(sema.KindLabel, true), "a label inside a function")
		})

		t.Run("keeps every binding with BindAll", func(t *testing.T) {
			t.Parallel()
			for _, kind := range sema.Kinds() {
				assert.True(t, engine.BindAll.Keeps(kind, true), kind.String())
			}
		})
	})
}
