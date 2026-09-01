// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import "go.dokimi.dev/techne/core/sema"

// Capture is a capture name in a tags query.
//
// The names are the ones the grammars' own tags queries already use, so
// a language module vendors its query unchanged rather than rewriting it
// to suit this engine.
type Capture string

const (
	// Name is the identifier belonging to a definition. It declares
	// nothing on its own.
	Name Capture = "name"

	DefinitionFunction  Capture = "definition.function"
	DefinitionMethod    Capture = "definition.method"
	DefinitionType      Capture = "definition.type"
	DefinitionClass     Capture = "definition.class"
	DefinitionInterface Capture = "definition.interface"
	DefinitionConstant  Capture = "definition.constant"
	DefinitionModule    Capture = "definition.module"
	DefinitionMacro     Capture = "definition.macro"
)

// kinds is the single definition point mapping a capture to what it
// declares.
//
// Two captures map onto one kind where a grammar draws a distinction the
// shared vocabulary does not carry. A class is a [sema.KindType] because
// no language served here needs the two told apart, and a macro is a
// [sema.KindFunction] because that is what it is invoked as. Adding a
// kind per grammar would make a caller read a different set depending on
// which language answered.
var kinds = map[Capture]sema.Kind{
	DefinitionFunction:  sema.KindFunction,
	DefinitionMethod:    sema.KindMethod,
	DefinitionType:      sema.KindType,
	DefinitionClass:     sema.KindType,
	DefinitionInterface: sema.KindInterface,
	DefinitionConstant:  sema.KindConstant,
	DefinitionModule:    sema.KindModule,
	DefinitionMacro:     sema.KindFunction,
}

// KindOf reports what a definition capture declares.
//
// It reports false for [Name], which belongs to a definition rather than
// being one, and for any capture no grammar declares. A mistyped capture
// must not map to a kind: an engine that silently finds nothing is the
// hardest failure to notice in a system whose job includes reporting
// that it found nothing.
func KindOf(c Capture) (sema.Kind, bool) {
	kind, declared := kinds[c]
	return kind, declared
}

// Definitions returns every capture that declares a symbol, so a caller
// can check a query against the set the engine understands.
func Definitions() []Capture {
	return []Capture{
		DefinitionFunction, DefinitionMethod, DefinitionType,
		DefinitionClass, DefinitionInterface, DefinitionConstant,
		DefinitionModule, DefinitionMacro,
	}
}
