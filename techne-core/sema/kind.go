// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "go.dokimi.dev/techne/core/internal/wire"

// Kind classifies a declaration. The zero value is KindUnknown. Each
// language module maps the declaration kinds of its grammar onto the closest
// Kind, so a caller reads the same values for every language.
type Kind uint8

const (
	// KindUnknown is an unclassified declaration.
	KindUnknown Kind = iota
	// KindModule is a named body of code, such as a Rust mod, a TypeScript
	// namespace, or a Ruby or Java module.
	KindModule
	// KindPackage is a Java or Scala package declaration.
	KindPackage
	// KindFile is a source file.
	KindFile
	// KindType is a named type that fits no other kind, such as an alias.
	KindType
	// KindStruct is a named aggregate of fields and methods: a struct in Go
	// and Rust, a class or object in Scala, and a class elsewhere.
	KindStruct
	// KindUnion is a Rust or C union. A TypeScript union type is KindType.
	KindUnion
	// KindEnum is an enumeration.
	KindEnum
	// KindEnumMember is a value declared inside an enumeration.
	KindEnumMember
	// KindInterface is an interface, trait, or protocol.
	KindInterface
	// KindAnnotation is an annotation type declaration, such as a Java
	// @interface. An annotation applied to a declaration is an Annotation.
	KindAnnotation
	// KindFunction is a function that is not a method.
	KindFunction
	// KindMethod is a function declared on a type.
	KindMethod
	// KindConstructor constructs instances of its type.
	KindConstructor
	// KindProperty is a field-like accessor: a TypeScript get or set, a C#
	// property, or a Python @property.
	KindProperty
	// KindMacro is a Rust declarative macro or a C #define.
	KindMacro
	// KindImplementation is a block that adds behaviour to a type, such as
	// a Rust impl block.
	KindImplementation
	// KindField is a member variable of a type.
	KindField
	// KindVariable is a variable.
	KindVariable
	// KindConstant is a constant.
	KindConstant
	// KindParameter is a parameter of a callable.
	KindParameter
	// KindTypeParameter is a type parameter, a Rust lifetime, or a const
	// generic.
	KindTypeParameter
	// KindImport is a name bound by an import.
	KindImport
	// KindLabel is a statement label.
	KindLabel
)

// kindWords are the wire strings of the kinds. IDs embed them, so changing
// one invalidates every stored ID.
var kindWords = wire.New(KindUnknown, map[Kind]string{
	KindUnknown:        "unknown",
	KindModule:         "module",
	KindPackage:        "package",
	KindFile:           "file",
	KindType:           "type",
	KindStruct:         "struct",
	KindUnion:          "union",
	KindEnum:           "enum",
	KindEnumMember:     "enum-member",
	KindInterface:      "interface",
	KindAnnotation:     "annotation",
	KindFunction:       "function",
	KindMethod:         "method",
	KindConstructor:    "constructor",
	KindProperty:       "property",
	KindMacro:          "macro",
	KindImplementation: "implementation",
	KindField:          "field",
	KindVariable:       "variable",
	KindConstant:       "constant",
	KindParameter:      "parameter",
	KindTypeParameter:  "type-parameter",
	KindImport:         "import",
	KindLabel:          "label",
})

// String returns the wire string of k, or "unknown" if k is not a declared
// Kind.
func (k Kind) String() string { return kindWords.String(k) }

// MarshalJSON encodes k as its wire string.
func (k Kind) MarshalJSON() ([]byte, error) { return kindWords.Marshal(k) }

// UnmarshalJSON decodes a wire string. An unknown string decodes to
// KindUnknown, so answers that use newer kinds still decode.
func (k *Kind) UnmarshalJSON(b []byte) error { return kindWords.Unmarshal(b, k) }

// Kinds returns every Kind except KindUnknown, in declaration order.
func Kinds() []Kind {
	return []Kind{
		KindModule, KindPackage, KindFile,
		KindType, KindStruct, KindUnion, KindEnum, KindEnumMember,
		KindInterface, KindAnnotation,
		KindFunction, KindMethod, KindConstructor, KindProperty,
		KindMacro, KindImplementation,
		KindField, KindVariable, KindConstant,
		KindParameter, KindTypeParameter,
		KindImport, KindLabel,
	}
}

// Declares reports whether code outside the declaring scope can refer to a
// declaration of kind k by name. It returns false for parameters, type
// parameters, labels, imports, and KindUnknown.
func (k Kind) Declares() bool {
	switch k {
	case KindParameter, KindTypeParameter, KindLabel, KindImport, KindUnknown:
		return false
	default:
		return true
	}
}
