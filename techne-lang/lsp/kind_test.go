// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp"
	"go.lsp.dev/protocol"
)

// valueKinds are the kinds of LSP 3.17 that declare nothing: the kinds of a value in a JSON
// document, and an event.
var valueKinds = map[protocol.SymbolKind]bool{
	protocol.SymbolKindString:  true,
	protocol.SymbolKindNumber:  true,
	protocol.SymbolKindBoolean: true,
	protocol.SymbolKindArray:   true,
	protocol.SymbolKindObject:  true,
	protocol.SymbolKindKey:     true,
	protocol.SymbolKindNull:    true,
	protocol.SymbolKindEvent:   true,
}

func TestKindOf(t *testing.T) {
	t.Parallel()

	t.Run("KindOf", func(t *testing.T) {
		t.Parallel()

		t.Run("maps a kind that declares something", func(t *testing.T) {
			t.Parallel()
			for kind, want := range map[protocol.SymbolKind]sema.Kind{
				protocol.SymbolKindStruct:     sema.KindStruct,
				protocol.SymbolKindClass:      sema.KindStruct,
				protocol.SymbolKindInterface:  sema.KindInterface,
				protocol.SymbolKindMethod:     sema.KindMethod,
				protocol.SymbolKindFunction:   sema.KindFunction,
				protocol.SymbolKindField:      sema.KindField,
				protocol.SymbolKindConstant:   sema.KindConstant,
				protocol.SymbolKindEnumMember: sema.KindEnumMember,
			} {
				got, declares := lsp.KindOf(kind)
				assert.True(t, declares, "KindOf reports that the kind declares something")
				assert.Equal(t, got, want, "the sema.Kind of the protocol kind")
			}
		})

		t.Run("maps no kind to KindUnknown", func(t *testing.T) {
			t.Parallel()
			for kind := protocol.SymbolKindFile; kind <= protocol.SymbolKindTypeParameter; kind++ {
				if got, declares := lsp.KindOf(kind); declares {
					assert.NotEqual(t, got, sema.KindUnknown, "the sema.Kind of a mapped protocol kind")
				}
			}
		})

		t.Run("maps every kind of the protocol except the value kinds", func(t *testing.T) {
			t.Parallel()
			for kind := protocol.SymbolKindFile; kind <= protocol.SymbolKindTypeParameter; kind++ {
				_, declares := lsp.KindOf(kind)
				assert.Equal(t, declares, !valueKinds[kind], "KindOf reports the protocol kind as a declaration")
			}
		})

		t.Run("reports false for a kind the protocol does not define", func(t *testing.T) {
			t.Parallel()
			_, declares := lsp.KindOf(protocol.SymbolKind(999))
			assert.False(t, declares, "KindOf of kind 999 reports a declaration")
		})
	})
}
