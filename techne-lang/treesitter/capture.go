// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import "go.dokimi.dev/techne/core/sema"

// Capture is a capture name in a tags query.
//
// The names follow the convention the grammars' own tags queries use. A
// module starts from upstream's query and extends it, because upstream
// aims at code navigation and stops well short of the vocabulary: the
// Go query tags a constant's name without saying it declares one, and
// the Rust query calls a struct, an enum, a union and a type alias all
// the same thing.
type Capture string

const (
	// Name is the identifier belonging to a definition. It declares
	// nothing on its own.
	Name Capture = "name"

	DefinitionFunction      Capture = "definition.function"
	DefinitionMethod        Capture = "definition.method"
	DefinitionConstructor   Capture = "definition.constructor"
	DefinitionType          Capture = "definition.type"
	DefinitionStruct        Capture = "definition.struct"
	DefinitionUnion         Capture = "definition.union"
	DefinitionClass         Capture = "definition.class"
	DefinitionObject        Capture = "definition.object"
	DefinitionEnum          Capture = "definition.enum"
	DefinitionEnumMember    Capture = "definition.enum_member"
	DefinitionInterface     Capture = "definition.interface"
	DefinitionAnnotation    Capture = "definition.annotation"
	DefinitionField         Capture = "definition.field"
	DefinitionProperty      Capture = "definition.property"
	DefinitionVariable      Capture = "definition.variable"
	DefinitionConstant      Capture = "definition.constant"
	DefinitionParameter     Capture = "definition.parameter"
	DefinitionTypeParameter Capture = "definition.type_parameter"
	DefinitionImport        Capture = "definition.import"
	DefinitionLabel         Capture = "definition.label"
	DefinitionPackage       Capture = "definition.package"
	DefinitionModule        Capture = "definition.module"
	DefinitionMacro         Capture = "definition.macro"
	DefinitionImplement     Capture = "definition.implementation"
)

// DefinitionPrefix marks a capture as one that declares something. A
// capture carrying it that the vocabulary does not know is a mistake in
// a query rather than a capture to pass over, and [New] refuses it.
const DefinitionPrefix = "definition."

// kinds is the single definition point mapping a capture to what it
// declares.
//
// Two captures map onto one kind where a grammar draws a distinction the
// shared vocabulary does not carry. A class is a [sema.KindStruct]: a
// Java class and a Go struct are one shape here, a named aggregate of
// fields and methods, and mapping them apart would mean a caller
// searching for that shape had to know which language answered. A Scala
// object is a singleton, which is that same shape.
//
// [sema.KindType] is what is left over: an alias, a bound, a type-level
// expression.
var kinds = map[Capture]sema.Kind{
	DefinitionFunction:      sema.KindFunction,
	DefinitionMethod:        sema.KindMethod,
	DefinitionConstructor:   sema.KindConstructor,
	DefinitionType:          sema.KindType,
	DefinitionStruct:        sema.KindStruct,
	DefinitionUnion:         sema.KindUnion,
	DefinitionClass:         sema.KindStruct,
	DefinitionObject:        sema.KindStruct,
	DefinitionEnum:          sema.KindEnum,
	DefinitionEnumMember:    sema.KindEnumMember,
	DefinitionInterface:     sema.KindInterface,
	DefinitionAnnotation:    sema.KindAnnotation,
	DefinitionField:         sema.KindField,
	DefinitionProperty:      sema.KindProperty,
	DefinitionVariable:      sema.KindVariable,
	DefinitionConstant:      sema.KindConstant,
	DefinitionParameter:     sema.KindParameter,
	DefinitionTypeParameter: sema.KindTypeParameter,
	DefinitionImport:        sema.KindImport,
	DefinitionLabel:         sema.KindLabel,
	DefinitionPackage:       sema.KindPackage,
	DefinitionModule:        sema.KindModule,
	DefinitionMacro:         sema.KindMacro,
	DefinitionImplement:     sema.KindImplementation,
}

// precedence ranks how much a kind says about a declaration.
//
// A query needs one pattern per shape, and a pattern cannot say what it
// is not: tree-sitter matches node types, and there is no way to write
// "a type_spec that is not a struct" or "a function not inside a class".
// The general pattern therefore also matches what the specific one
// matches, and both reach the engine for one declaration. The higher
// rank is the one kept.
//
// Kinds absent here rank zero, which is right for [sema.KindUnknown]: it
// never displaces anything, and anything displaces it.
var precedence = map[sema.Kind]int{
	sema.KindType: 1,

	// A binding that does not leave its scope never displaces a
	// declaration that does, so these sit below everything else.
	sema.KindTypeParameter: 1,
	sema.KindImport:        1,
	sema.KindLabel:         1,

	sema.KindFunction: 2,
	sema.KindVariable: 2,

	// A destructured parameter matches the general pattern for a
	// destructured binding as well as the one for a parameter. Inside a
	// signature, parameter is the more specific answer; nowhere else do
	// the two compete.
	sema.KindParameter: 3,

	sema.KindStruct:         4,
	sema.KindUnion:          4,
	sema.KindInterface:      4,
	sema.KindEnum:           4,
	sema.KindAnnotation:     4,
	sema.KindMethod:         4,
	sema.KindField:          4,
	sema.KindEnumMember:     4,
	sema.KindConstructor:    4,
	sema.KindModule:         4,
	sema.KindPackage:        4,
	sema.KindProperty:       4,
	sema.KindMacro:          4,
	sema.KindImplementation: 4,

	// A constant outranks the field or variable it is also spelled as.
	// Java writes one as `static final`, so both patterns match, and a
	// tie would leave the kind to whichever match arrived first.
	sema.KindConstant: 5,
}

// Outranks reports whether one kind says more about a declaration than
// another, and so should replace it.
func Outranks(candidate, held sema.Kind) bool {
	return precedence[candidate] > precedence[held]
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
		DefinitionFunction, DefinitionMethod, DefinitionConstructor,
		DefinitionType, DefinitionStruct, DefinitionUnion,
		DefinitionClass, DefinitionObject,
		DefinitionEnum, DefinitionEnumMember, DefinitionInterface,
		DefinitionAnnotation,
		DefinitionField, DefinitionProperty,
		DefinitionVariable, DefinitionConstant,
		DefinitionParameter, DefinitionTypeParameter,
		DefinitionImport, DefinitionLabel,
		DefinitionPackage, DefinitionModule, DefinitionMacro,
		DefinitionImplement,
	}
}
