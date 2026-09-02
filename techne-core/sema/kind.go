// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "encoding/json"

// Kind is what a declaration is.
//
// The set is drawn from what the grammars distinguish rather than from
// what seemed likely: every value is a node kind at least one supported
// language declares. It is still smaller than any one language's
// grammar, and a distinction only one language draws is not carried.
// That language maps both sides onto the nearest value, so a caller
// reads the same set whichever language answered.
//
// The zero value is [KindUnknown].
type Kind uint8

const (
	// KindUnknown means the engine could not classify the declaration.
	KindUnknown Kind = iota
	// KindModule is a named body of code: a Rust mod, a TypeScript
	// namespace, a Ruby module, a Java module.
	KindModule
	// KindPackage is the package a file belongs to, as Java's and
	// Scala's package statements declare it.
	KindPackage
	KindFile
	// KindType is a named type that is none of the shapes below it: an
	// alias, a bound, a type-level expression.
	KindType
	// KindStruct is a named aggregate of fields and methods. A class is
	// one: Java, Python, Ruby and TypeScript spell it class, Go and Rust
	// spell it struct, and Scala also spells it object. Telling those
	// apart would mean a caller had to know which language answered
	// before it could ask a question.
	KindStruct
	// KindUnion is a type holding one of several shapes at a time. Rust
	// and C spell it union. A TypeScript union type is a [KindType],
	// because it is an expression rather than a declared body.
	KindUnion
	KindEnum
	// KindEnumMember is a value declared inside an enum. It is not a
	// constant: it is scoped to its enum and carries that enum's type.
	KindEnumMember
	KindInterface
	// KindAnnotation is a declared annotation type, as Java's
	// @interface declares one. Applying an annotation declares nothing
	// and arrives as an [Annotation] on the thing annotated.
	KindAnnotation
	KindFunction
	KindMethod
	// KindConstructor builds an instance. Renaming one follows its
	// type's name rather than being free, which is why it is not a
	// method.
	KindConstructor
	// KindProperty is an accessor that reads as a field: a TypeScript
	// get or set, a C# property, a Python @property.
	KindProperty
	// KindMacro is expanded before compilation rather than called. Rust
	// declares one with macro_rules!, C with #define.
	KindMacro
	// KindImplementation is a block attaching behaviour to a type, as
	// Rust's impl does. It names the type it is for.
	KindImplementation
	KindField
	KindVariable
	KindConstant
	// KindParameter is a binding in a callable's signature.
	KindParameter
	// KindTypeParameter is a generic parameter: a type variable, a Rust
	// lifetime, a const generic.
	KindTypeParameter
	// KindImport brings a name into scope. It declares that name
	// locally, which is why it is a declaration rather than a reference.
	KindImport
	// KindLabel names a statement so control flow can target it.
	KindLabel
)

// kindNames is the single definition point for the wire form of each
// kind. [NewID] embeds these strings, so an identity an index stored
// changes meaning if one of them changes.
var kindNames = map[Kind]string{
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
}

// String returns the wire form of the kind.
//
// A value outside the declared set returns the wire form of
// [KindUnknown] rather than an empty string, so a malformed identity
// stays parseable and does not collide with every other malformed one.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return kindNames[KindUnknown]
}

// MarshalJSON writes the wire form rather than the number, so a caller
// reads a kind without a lookup table.
func (k Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON reads the wire form back.
//
// A word this package does not know becomes [KindUnknown] rather than an
// error. A vocabulary that grows is one where an older reader meets a
// newer word, and refusing the whole answer over one field it does not
// recognise loses everything else in it.
func (k *Kind) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	*k = KindUnknown
	for kind, held := range kindNames {
		if held == name {
			*k = kind
			return nil
		}
	}
	return nil
}

// Kinds returns every kind the vocabulary carries, [KindUnknown]
// excepted, so a caller checks a value against the set rather than
// against a list of its own.
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

// Declares reports whether a kind names something other code can refer
// to by name.
//
// A parameter, a type parameter, a label and an import bind names that
// do not leave the scope declaring them, so a caller listing what a file
// offers drops all four in one check rather than naming each.
func (k Kind) Declares() bool {
	switch k {
	case KindParameter, KindTypeParameter, KindLabel, KindImport, KindUnknown:
		return false
	default:
		return true
	}
}
