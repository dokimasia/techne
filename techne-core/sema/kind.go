// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

// Kind is what a declaration is.
//
// The set is deliberately smaller than any one language's grammar: it is
// the vocabulary every language maps onto, not the union of all of them.
// A language whose grammar draws a distinction this set does not carry
// maps both sides onto the nearest kind rather than gaining a value.
//
// The zero value is [KindUnknown].
type Kind uint8

const (
	// KindUnknown means the engine could not classify the declaration.
	KindUnknown Kind = iota
	KindModule
	KindPackage
	KindFile
	KindType
	KindInterface
	KindFunction
	KindMethod
	KindField
	KindVariable
	KindConstant
)

// kindNames is the single definition point for the wire form of each
// kind. [NewID] embeds these strings, so an identity an index stored
// changes meaning if one of them changes.
var kindNames = map[Kind]string{
	KindUnknown:   "unknown",
	KindModule:    "module",
	KindPackage:   "package",
	KindFile:      "file",
	KindType:      "type",
	KindInterface: "interface",
	KindFunction:  "function",
	KindMethod:    "method",
	KindField:     "field",
	KindVariable:  "variable",
	KindConstant:  "constant",
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
