// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import "go.dokimi.dev/techne/core/trust"

// ArgKey names a parameter. Every string that crosses this boundary is a
// constant with one definition point: a mistyped key produces an
// operation that silently does nothing, which is the hardest failure to
// notice in a system whose job includes reporting that it found nothing.
type ArgKey string

const (
	ArgNewName     ArgKey = "new_name"
	ArgDestination ArgKey = "destination"
	ArgSignature   ArgKey = "signature"
	ArgReceiver    ArgKey = "receiver"
	ArgDoc         ArgKey = "doc"
)

// Args are the parameters of one request.
type Args map[ArgKey]string

// Spec is what an operation declares about itself.
type Spec struct {
	Operation Operation
	// Accepts lists the target kinds the operation can be pointed at. An
	// empty list would mean nothing can invoke it.
	Accepts  []TargetKind
	Required []ArgKey
	Optional []ArgKey

	// MinFidelity is the weakest binding this operation can be correct
	// on. The policy compares it against what the planner actually had.
	MinFidelity trust.Fidelity

	// RewritesReferences says the operation changes code that refers to
	// the target. Admission then also demands total coverage, because
	// finding every reference is a claim that no others exist, and that
	// is a negative claim.
	RewritesReferences bool
}

// specs is the table Operations is checked against. Adding an operation
// without adding a row here fails the package's tests.
var specs = map[Operation]Spec{
	RenameSymbol: {
		Operation: RenameSymbol, Accepts: []TargetKind{TargetSymbol},
		Required: []ArgKey{ArgNewName}, MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	RenameFile: {
		Operation: RenameFile, Accepts: []TargetKind{TargetFile},
		Required: []ArgKey{ArgNewName}, MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	MoveFile: {
		Operation: MoveFile, Accepts: []TargetKind{TargetFile},
		Required: []ArgKey{ArgDestination}, MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	MoveSymbol: {
		Operation: MoveSymbol, Accepts: []TargetKind{TargetSymbol},
		Required: []ArgKey{ArgDestination}, MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	ExtractFunction: {
		Operation: ExtractFunction, Accepts: []TargetKind{TargetSpan},
		Required: []ArgKey{ArgNewName}, MinFidelity: trust.Resolved,
	},
	ExtractVariable: {
		Operation: ExtractVariable, Accepts: []TargetKind{TargetSpan},
		Required: []ArgKey{ArgNewName}, MinFidelity: trust.Resolved,
	},
	ExtractInterface: {
		Operation: ExtractInterface, Accepts: []TargetKind{TargetSymbol},
		Required: []ArgKey{ArgNewName}, MinFidelity: trust.Resolved,
	},
	InlineVariable: {
		Operation: InlineVariable, Accepts: []TargetKind{TargetSymbol},
		MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	InlineConstant: {
		Operation: InlineConstant, Accepts: []TargetKind{TargetSymbol},
		MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	ChangeSignature: {
		Operation: ChangeSignature, Accepts: []TargetKind{TargetSymbol},
		Required: []ArgKey{ArgSignature}, MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	ImplementInterface: {
		Operation: ImplementInterface, Accepts: []TargetKind{TargetSymbol},
		Optional: []ArgKey{ArgReceiver}, MinFidelity: trust.Resolved,
	},
	// document.symbol writes a comment above a declaration and touches
	// nothing else, which is why a parser can serve it correctly. It is
	// the one operation whose minimum is not resolved, and the reason
	// the minimum is a property of the operation rather than a setting.
	//
	// It is also the one that takes a span as readily as a symbol. A
	// name and a kind do not pick out one declaration — a package with
	// two Get methods has two symbols under one identity — and the text
	// to write is the caller's, so a caller that has already resolved
	// which declaration it means says so by its position.
	DocumentSymbol: {
		Operation: DocumentSymbol, Accepts: []TargetKind{TargetSymbol, TargetSpan},
		Required: []ArgKey{ArgDoc}, MinFidelity: trust.Syntactic,
	},
}

// SpecFor returns what an operation declares about itself, and reports
// whether the operation is declared at all.
func SpecFor(o Operation) (Spec, bool) {
	s, ok := specs[o]
	return s, ok
}
