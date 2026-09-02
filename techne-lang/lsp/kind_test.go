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

// valueKinds are the kinds the protocol carries because the same request
// outlines a JSON document. They are the whole of what this package
// drops on purpose, so the completeness case below can tell a deliberate
// omission from a forgotten one.
var valueKinds = map[protocol.SymbolKind]bool{
	protocol.SymbolKindString:  true,
	protocol.SymbolKindNumber:  true,
	protocol.SymbolKindBoolean: true,
	protocol.SymbolKindArray:   true,
	protocol.SymbolKindObject:  true,
	protocol.SymbolKindKey:     true,
	protocol.SymbolKindNull:    true,
	// An event is something a declaration raises rather than something
	// declared, and this vocabulary has no member for it.
	protocol.SymbolKindEvent: true,
}

func TestKindOf(t *testing.T) {
	t.Parallel()

	t.Run("a kind that declares something", func(t *testing.T) {
		t.Parallel()

		t.Run("maps onto what this vocabulary calls it", func(t *testing.T) {
			t.Parallel()
			for held, want := range map[protocol.SymbolKind]sema.Kind{
				protocol.SymbolKindStruct:     sema.KindStruct,
				protocol.SymbolKindClass:      sema.KindStruct,
				protocol.SymbolKindInterface:  sema.KindInterface,
				protocol.SymbolKindMethod:     sema.KindMethod,
				protocol.SymbolKindFunction:   sema.KindFunction,
				protocol.SymbolKindField:      sema.KindField,
				protocol.SymbolKindConstant:   sema.KindConstant,
				protocol.SymbolKindEnumMember: sema.KindEnumMember,
			} {
				got, declares := lsp.KindOf(held)
				assert.True(t, declares, "the protocol and this vocabulary both carry it")
				assert.Equal(t, got, want, "under the name this one uses")
			}
		})

		t.Run("is never reported as unknown", func(t *testing.T) {
			t.Parallel()
			// Answering with the unknown member would put an entry in an
			// outline that names a declaration a caller cannot filter on
			// or act on, which is worse than leaving it out.
			for held := protocol.SymbolKindFile; held <= protocol.SymbolKindTypeParameter; held++ {
				got, declares := lsp.KindOf(held)
				if !declares {
					continue
				}
				assert.NotEqual(t, got, sema.KindUnknown,
					"a kind that maps at all maps onto a member this vocabulary carries")
			}
		})
	})

	t.Run("a kind that declares nothing", func(t *testing.T) {
		t.Parallel()

		t.Run("is dropped rather than reported as unknown", func(t *testing.T) {
			t.Parallel()
			for held := range valueKinds {
				_, declares := lsp.KindOf(held)
				assert.False(t, declares, "a value in a document is not a declaration")
			}
		})

		t.Run("includes one the protocol adds later", func(t *testing.T) {
			t.Parallel()
			_, declares := lsp.KindOf(protocol.SymbolKind(999))
			assert.False(t, declares,
				"a kind this vocabulary has never heard of is left out rather than guessed at")
		})
	})

	t.Run("the kinds the specification defines", func(t *testing.T) {
		t.Parallel()

		t.Run("are every one classified", func(t *testing.T) {
			t.Parallel()
			// Walked between the protocol's own first and last constants
			// rather than between numbers written here, so a kind added
			// inside that range arrives as a failure rather than as a
			// declaration silently missing from every outline. A kind
			// added past the last one is invisible to this, and is what
			// [valueKinds] would have to grow for.
			for held := protocol.SymbolKindFile; held <= protocol.SymbolKindTypeParameter; held++ {
				_, declares := lsp.KindOf(held)
				assert.Equal(t, declares, !valueKinds[held],
					"a kind is mapped or dropped on purpose, never by omission")
			}
		})
	})
}
