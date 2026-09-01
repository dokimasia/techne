// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// TestDoc covers the contracts the package comment states across more
// than one declaration.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("identity", func(t *testing.T) {
		t.Parallel()

		t.Run("is independent of where the declaration sits", func(t *testing.T) {
			t.Parallel()
			// Two engines reading the same declaration from files whose
			// unrelated lines differ must agree on its identity, or an
			// index cannot be shared between them.
			id := sema.NewID(source.Go, "./core/sema", "Symbol", sema.KindType)
			early := sema.Symbol{
				ID:   id,
				Span: source.Span{Path: "core/sema/symbol.go", Start: source.Position{Offset: 100, Line: 8}},
			}
			late := sema.Symbol{
				ID:   id,
				Span: source.Span{Path: "core/sema/symbol.go", Start: source.Position{Offset: 4200, Line: 210}},
			}
			if early.ID != late.ID {
				t.Errorf("the same declaration at two offsets produced %q and %q", early.ID, late.ID)
			}
		})
	})

	t.Run("directions", func(t *testing.T) {
		t.Parallel()

		t.Run("let a service turn any stored edge around", func(t *testing.T) {
			t.Parallel()
			// An engine stores Calls and a caller asks for CalledBy. The
			// service swaps the ends and inverts the kind, so the two
			// have to describe the same edge.
			stored := sema.Relation{Kind: sema.Calls, From: "go:./a#F:function", To: "go:./b#G:function"}
			turned := sema.Relation{Kind: stored.Kind.Inverse(), From: stored.To, To: stored.From}
			if turned.Kind.Inverse() != stored.Kind {
				t.Errorf("turning %d around gave %d, which does not invert back", stored.Kind, turned.Kind)
			}
			if turned.From != stored.To || turned.To != stored.From {
				t.Error("turning the edge around did not swap its ends")
			}
		})
	})
}
