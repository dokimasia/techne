// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import "strings"

// Family is the verb an operation belongs to. A family can be advertised
// or refused as a unit.
type Family string

const (
	FamilyRename    Family = "rename"
	FamilyMove      Family = "move"
	FamilyExtract   Family = "extract"
	FamilyInline    Family = "inline"
	FamilyChange    Family = "change"
	FamilyImplement Family = "implement"
	FamilyDocument  Family = "document"
)

// Operation is one thing a caller can ask for, named family.subject.
type Operation string

const (
	RenameSymbol       Operation = "rename.symbol"
	RenameFile         Operation = "rename.file"
	MoveFile           Operation = "move.file"
	MoveSymbol         Operation = "move.symbol"
	ExtractFunction    Operation = "extract.function"
	ExtractVariable    Operation = "extract.variable"
	ExtractInterface   Operation = "extract.interface"
	InlineVariable     Operation = "inline.variable"
	InlineConstant     Operation = "inline.constant"
	ChangeSignature    Operation = "change.signature"
	ImplementInterface Operation = "implement.interface"
	DocumentSymbol     Operation = "document.symbol"
)

// Family returns the part before the dot, or the whole name when an
// operation has no subject.
func (o Operation) Family() Family {
	name, _, found := strings.Cut(string(o), ".")
	if !found {
		return Family(o)
	}
	return Family(name)
}

// Operations returns every declared operation, in catalogue order.
//
// It is the list the spec table is checked against, so an operation
// added here without a spec fails the package's tests rather than
// reaching a caller with nothing to validate it.
func Operations() []Operation {
	return []Operation{
		RenameSymbol, RenameFile,
		MoveFile, MoveSymbol,
		ExtractFunction, ExtractVariable, ExtractInterface,
		InlineVariable, InlineConstant,
		ChangeSignature,
		ImplementInterface,
		DocumentSymbol,
	}
}
