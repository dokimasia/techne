// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
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
			id := sema.NewID(source.Language("go"), "./core/sema", "Symbol", sema.KindType)
			early := sema.Symbol{
				ID:   id,
				Span: source.Span{Path: "core/sema/symbol.go", Start: source.Position{Offset: 100, Line: 8}},
			}
			late := sema.Symbol{
				ID:   id,
				Span: source.Span{Path: "core/sema/symbol.go", Start: source.Position{Offset: 4200, Line: 210}},
			}
			assert.Equal(t, early.ID, late.ID,
				"two engines reading one declaration at different offsets must agree on its identity")
		})
	})

	t.Run("directions", func(t *testing.T) {
		t.Parallel()

		t.Run("let a service turn any stored edge around", func(t *testing.T) {
			t.Parallel()
			stored := sema.Relation{Kind: sema.Calls, From: "go:./a#F:function", To: "go:./b#G:function"}
			turned := sema.Relation{Kind: stored.Kind.Inverse(), From: stored.To, To: stored.From}

			assert.Equal(t, turned.Kind.Inverse(), stored.Kind,
				"turning an edge around and back describes the edge that was stored")
			assert.Equal(t, turned.From, stored.To, "turning an edge around swaps its ends")
			assert.Equal(t, turned.To, stored.From, "turning an edge around swaps its ends")
		})
	})
}
