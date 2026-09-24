// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

// Operation names an operation that a caller can request, in the form verb.subject, such as
// rename.symbol.
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

// Operations returns every declared operation, grouped by verb. Each call returns a new slice.
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
