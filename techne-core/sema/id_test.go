// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

func TestID(t *testing.T) {
	t.Parallel()

	t.Run("NewID", func(t *testing.T) {
		t.Parallel()

		t.Run("is language, unit, qualified name and kind", func(t *testing.T) {
			t.Parallel()
			// The language is an example value. A language module owns
			// its own, and core declares none.
			got := sema.NewID(source.Language("go"), "./internal/fsx", "Digest", sema.KindFunction)
			assert.Equal(t, got, sema.ID("go:./internal/fsx#Digest:function"),
				"an identity states which language, which unit, which name and which kind")
		})

		t.Run("does not change when the declaration moves", func(t *testing.T) {
			t.Parallel()
			first := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindType)
			second := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindType)
			assert.Equal(t, first, second,
				"an index stores an identity, so editing a line above a declaration must not invalidate it")
		})

		t.Run("separates a type from a function of the same name", func(t *testing.T) {
			t.Parallel()
			asType := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindType)
			asFunc := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindFunction)
			assert.NotEqual(t, asType, asFunc, "two declarations are two symbols, however alike their names")
		})

		t.Run("separates one name in two units", func(t *testing.T) {
			t.Parallel()
			inTrust := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindType)
			inGate := sema.NewID(source.Language("go"), "./core/gate", "Status", sema.KindType)
			assert.NotEqual(t, inTrust, inGate, "a name is only unique inside its unit")
		})

		t.Run("separates one name in two languages", func(t *testing.T) {
			t.Parallel()
			inGo := sema.NewID(source.Language("go"), "./app", "Handler", sema.KindType)
			inRust := sema.NewID(source.Language("rust"), "./app", "Handler", sema.KindType)
			assert.NotEqual(t, inGo, inRust, "two languages in one tree declare two symbols")
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("names nothing", func(t *testing.T) {
			t.Parallel()
			var unset sema.ID
			assert.Empty(t, string(unset), "an unset identity names no declaration")
		})
	})
}
