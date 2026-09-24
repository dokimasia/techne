// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import "go.dokimi.dev/techne/core/sema"

// Capture is a capture name of a tags query. The names follow the
// convention of the tags queries in the grammar repositories, extended with
// the kinds those queries do not distinguish, such as constants and unions.
type Capture string

const (
	// Name captures the identifier of a definition. It defines nothing on
	// its own.
	Name Capture = "name"

	// Receiver captures the name of the type that a definition belongs to
	// when the syntax writes the definition outside the type, as Go writes a
	// method. The type is the container in the qualified name of the
	// definition. It defines nothing on its own.
	Receiver Capture = "receiver"

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

// DefinitionPrefix starts every definition capture. [New] refuses a query
// with a capture that starts with it and has no kind.
const DefinitionPrefix = "definition."

// kinds maps each definition capture to the kind it defines, with
// [sema.KindStruct] for classes as well as structs. A caller finds the named
// aggregate of fields and methods by one kind in every language. A Scala
// object is [sema.KindModule], as metals reports it, so a class and its
// companion object differ by kind. [sema.KindType] covers aliases, bounds
// and type expressions.
var kinds = map[Capture]sema.Kind{
	DefinitionFunction:      sema.KindFunction,
	DefinitionMethod:        sema.KindMethod,
	DefinitionConstructor:   sema.KindConstructor,
	DefinitionType:          sema.KindType,
	DefinitionStruct:        sema.KindStruct,
	DefinitionUnion:         sema.KindUnion,
	DefinitionClass:         sema.KindStruct,
	DefinitionObject:        sema.KindModule,
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

// precedence contains the rank of each kind for [MoreSpecific]. A pattern
// cannot exclude the shapes of a more specific pattern, so two patterns can
// match one declaration, and the declaration takes the kind with the higher
// rank. A kind absent from the map ranks zero, which puts KindUnknown below
// every kind.
var precedence = map[sema.Kind]int{
	sema.KindType: 1,

	// Bindings local to a scope rank below every declaration.
	sema.KindTypeParameter: 1,
	sema.KindImport:        1,
	sema.KindLabel:         1,

	sema.KindFunction: 2,
	sema.KindVariable: 2,

	// A destructured parameter also matches the pattern of a destructured
	// binding.
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

	// A Java constant, written as a static final field, also matches the
	// field pattern.
	sema.KindConstant: 5,
}

// MoreSpecific reports whether candidate is more specific than current, so
// that candidate replaces current as the kind of one declaration.
func MoreSpecific(candidate, current sema.Kind) bool {
	return precedence[candidate] > precedence[current]
}

// KindOf returns the kind that a definition capture defines. It reports
// false for [Name] and for any capture without a kind.
func KindOf(c Capture) (sema.Kind, bool) {
	kind, ok := kinds[c]
	return kind, ok
}

// Definitions returns every definition capture, so a caller can check a
// query against the captures the engine reads.
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
