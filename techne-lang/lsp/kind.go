// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"go.dokimi.dev/techne/core/sema"
	"go.lsp.dev/protocol"
)

// kinds maps what a server says a declaration is onto what techne says.
//
// Keyed by the protocol's own constants rather than by numbers written
// out here. The numbering is wire format, and a second copy of it is a
// copy that can drift from the one the answers are decoded with.
//
// Not every kind has a home. The protocol names the shapes a value takes
// in a document as well as the shapes a declaration takes in code — a
// string, a number, a key — because the same request outlines JSON.
// Those declare nothing here.
var kinds = map[protocol.SymbolKind]sema.Kind{
	protocol.SymbolKindFile:          sema.KindFile,
	protocol.SymbolKindModule:        sema.KindModule,
	protocol.SymbolKindNamespace:     sema.KindModule,
	protocol.SymbolKindPackage:       sema.KindPackage,
	protocol.SymbolKindClass:         sema.KindStruct,
	protocol.SymbolKindMethod:        sema.KindMethod,
	protocol.SymbolKindProperty:      sema.KindProperty,
	protocol.SymbolKindField:         sema.KindField,
	protocol.SymbolKindConstructor:   sema.KindConstructor,
	protocol.SymbolKindEnum:          sema.KindEnum,
	protocol.SymbolKindInterface:     sema.KindInterface,
	protocol.SymbolKindFunction:      sema.KindFunction,
	protocol.SymbolKindVariable:      sema.KindVariable,
	protocol.SymbolKindConstant:      sema.KindConstant,
	protocol.SymbolKindEnumMember:    sema.KindEnumMember,
	protocol.SymbolKindStruct:        sema.KindStruct,
	protocol.SymbolKindOperator:      sema.KindFunction,
	protocol.SymbolKindTypeParameter: sema.KindTypeParameter,
}

// KindOf reports what a declaration of this kind is, and whether it is
// one at all.
//
// A kind the protocol has and this vocabulary does not is dropped rather
// than reported as unknown. A caller filtering an outline would
// otherwise meet entries that declare nothing and cannot be acted on.
func KindOf(held protocol.SymbolKind) (sema.Kind, bool) {
	kind, declares := kinds[held]
	return kind, declares
}
