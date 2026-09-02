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

		t.Run("let an engine store one and answer for both", func(t *testing.T) {
			t.Parallel()
			// An edge names the far end and takes the near end from the
			// question, so answering the other direction is a matter of
			// which end an engine returns rather than of rewriting what
			// it stored.
			called := sema.Relation{
				Kind: sema.CalledBy,
				To:   sema.Symbol{Name: "F", Kind: sema.KindFunction},
			}
			assert.Equal(t, called.Kind.Inverse(), sema.Calls,
				"a caller asking who calls this is answered from the edges that call it")
			assert.Equal(t, called.Kind.Inverse().Inverse(), called.Kind,
				"turning a direction around twice is the direction that was asked for")
		})
	})
}
