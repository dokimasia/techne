// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import "strings"

// Family is the verb of an operation, such as rename or extract.
type Family string

// The families of the declared operations.
const (
	FamilyRename    Family = "rename"
	FamilyMove      Family = "move"
	FamilyExtract   Family = "extract"
	FamilyInline    Family = "inline"
	FamilyChange    Family = "change"
	FamilyImplement Family = "implement"
	FamilyDocument  Family = "document"
)

// Operation names an operation a caller can request, in the form
// family.subject.
type Operation string

// The declared operations. [SpecFor] returns the spec of each one.
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

// Family returns the part of o before the first dot, or o itself when it
// contains no dot.
func (o Operation) Family() Family {
	name, _, _ := strings.Cut(string(o), ".")
	return Family(name)
}

// Operations returns every declared operation, grouped by family. Each call
// returns a new slice.
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
