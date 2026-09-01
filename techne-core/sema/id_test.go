// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

func TestID(t *testing.T) {
	t.Parallel()

	t.Run("NewID", func(t *testing.T) {
		t.Parallel()

		t.Run("is language, unit, qualified name and kind", func(t *testing.T) {
			t.Parallel()
			got := sema.NewID(source.Go, "./internal/fsx", "Digest", sema.KindFunction)
			const want sema.ID = "go:./internal/fsx#Digest:function"
			if got != want {
				t.Errorf("NewID = %q, want %q", got, want)
			}
		})

		t.Run("does not change when the declaration moves", func(t *testing.T) {
			t.Parallel()
			// An index stores identities and a later process resolves
			// them. Derived from a byte offset, editing a line above a
			// declaration would invalidate every entry below it.
			first := sema.NewID(source.Go, "./core/trust", "Status", sema.KindType)
			second := sema.NewID(source.Go, "./core/trust", "Status", sema.KindType)
			if first != second {
				t.Errorf("one declaration produced %q then %q", first, second)
			}
		})

		t.Run("separates a type from a function of the same name", func(t *testing.T) {
			t.Parallel()
			asType := sema.NewID(source.Go, "./core/trust", "Status", sema.KindType)
			asFunc := sema.NewID(source.Go, "./core/trust", "Status", sema.KindFunction)
			if asType == asFunc {
				t.Errorf("a type and a function both produced %q", asType)
			}
		})

		t.Run("separates one name in two units", func(t *testing.T) {
			t.Parallel()
			inTrust := sema.NewID(source.Go, "./core/trust", "Status", sema.KindType)
			inGate := sema.NewID(source.Go, "./core/gate", "Status", sema.KindType)
			if inTrust == inGate {
				t.Errorf("two units both produced %q", inTrust)
			}
		})

		t.Run("separates one name in two languages", func(t *testing.T) {
			t.Parallel()
			inGo := sema.NewID(source.Go, "./app", "Handler", sema.KindType)
			inRust := sema.NewID(source.Rust, "./app", "Handler", sema.KindType)
			if inGo == inRust {
				t.Errorf("two languages both produced %q", inGo)
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("names nothing", func(t *testing.T) {
			t.Parallel()
			var unset sema.ID
			if unset != "" {
				t.Errorf("zero ID = %q, want the empty string", unset)
			}
		})
	})
}
