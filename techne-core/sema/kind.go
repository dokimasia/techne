// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

// Kind is what a declaration is.
//
// The set is smaller than any one language's grammar and larger than the
// weakest engine can tell apart. It is sized for the strongest engine:
// a parser distinguishes a handful of shapes, a language server
// distinguishes more, and a kind the vocabulary cannot express would
// make the stronger tier lossy for no reason.
//
// A distinction only one language draws is not carried. A language that
// makes one maps both sides onto the nearest kind, so a caller reads the
// same set whichever language answered.
//
// The zero value is [KindUnknown].
type Kind uint8

const (
	// KindUnknown means the engine could not classify the declaration.
	KindUnknown Kind = iota
	KindModule
	KindPackage
	KindFile
	// KindType is a named type that is none of the shapes below it: an
	// alias, a union, a trait bound.
	KindType
	KindStruct
	KindEnum
	// KindEnumMember is a value declared inside an enum. It is not a
	// constant: it is scoped to its enum and carries that enum's type.
	KindEnumMember
	KindInterface
	KindFunction
	KindMethod
	// KindConstructor builds an instance. Renaming one follows its
	// type's name rather than being free, which is why it is not a
	// method.
	KindConstructor
	KindField
	KindVariable
	KindConstant
)

// kindNames is the single definition point for the wire form of each
// kind. [NewID] embeds these strings, so an identity an index stored
// changes meaning if one of them changes.
var kindNames = map[Kind]string{
	KindUnknown:     "unknown",
	KindModule:      "module",
	KindPackage:     "package",
	KindFile:        "file",
	KindType:        "type",
	KindStruct:      "struct",
	KindEnum:        "enum",
	KindEnumMember:  "enum-member",
	KindInterface:   "interface",
	KindFunction:    "function",
	KindMethod:      "method",
	KindConstructor: "constructor",
	KindField:       "field",
	KindVariable:    "variable",
	KindConstant:    "constant",
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
