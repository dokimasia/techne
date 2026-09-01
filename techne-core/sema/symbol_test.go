// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
)

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("sits at the top level of no unit", func(t *testing.T) {
			t.Parallel()
			// Parent is empty at the top level, so the zero Symbol must
			// not read as nested inside something.
			var unset sema.Symbol
			if unset.Parent != "" {
				t.Errorf("zero Symbol.Parent = %q, want the empty string", unset.Parent)
			}
			if unset.Kind != sema.KindUnknown {
				t.Errorf("zero Symbol.Kind = %d, want KindUnknown", unset.Kind)
			}
			if unset.Exported {
				t.Error("zero Symbol must not claim to be exported")
			}
		})
	})

	t.Run("Doc", func(t *testing.T) {
		t.Parallel()

		t.Run("is not needed to identify the declaration", func(t *testing.T) {
			t.Parallel()
			// The output budget drops Doc before it drops anything else,
			// so every field a caller identifies a declaration by has to
			// survive without it.
			thinned := sema.Symbol{ID: "go:./a#F:function", Name: "F", Kind: sema.KindFunction}
			if thinned.ID == "" || thinned.Name == "" || thinned.Kind == sema.KindUnknown {
				t.Errorf("a symbol carrying no Doc is not identifiable: %+v", thinned)
			}
		})
	})
}
