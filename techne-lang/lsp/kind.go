// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"go.dokimi.dev/techne/core/sema"
	"go.lsp.dev/protocol"
)

// kinds maps each LSP 3.17 symbol kind that declares something to its [sema.Kind]. The kinds of
// a value in a document, such as a string, a number or a key in JSON, are absent, and so is an
// event.
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

// KindOf returns the [sema.Kind] of a protocol symbol kind, and reports whether the kind
// declares something. A kind without a mapping, such as a string in a JSON document or a kind
// that a later version of the protocol adds, reports false.
func KindOf(kind protocol.SymbolKind) (sema.Kind, bool) {
	mapped, declares := kinds[kind]
	return mapped, declares
}
