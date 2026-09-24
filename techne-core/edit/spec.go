// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import "go.dokimi.dev/techne/core/trust"

// ArgKey names an argument of an operation. [Spec.Required] and
// [Spec.Optional] list the keys each operation takes, and the write path
// refuses a request with any other key.
type ArgKey string

// The declared argument keys.
const (
	ArgNewName     ArgKey = "new_name"
	ArgDestination ArgKey = "destination"
	ArgSignature   ArgKey = "signature"
	ArgReceiver    ArgKey = "receiver"
	ArgDoc         ArgKey = "doc"
)

// Args are the arguments of one request.
type Args map[ArgKey]string

// Spec declares the targets, the arguments and the evidence an operation
// requires.
type Spec struct {
	Operation Operation
	// Accepts lists the target kinds the operation applies to. It is never
	// empty.
	Accepts []TargetKind
	// Required lists the argument keys a request must set.
	Required []ArgKey
	// Optional lists the argument keys a request can set.
	Optional []ArgKey
	// MinFidelity is the lowest tier of evidence on which the operation is
	// correct. [Policy.Admit] compares it with the fidelity of the plan.
	MinFidelity trust.Fidelity
	// RewritesReferences reports whether the operation rewrites the
	// references to its target. A plan for such an operation also requires
	// evidence that supports a negative claim, because it asserts that no
	// other references exist.
	RewritesReferences bool
}

// pointed lists the target kinds of an operation on one declaration: an ID
// or a span. An ID matches two declarations when one unit declares two
// methods named Get, and a span selects exactly one of them.
var pointed = []TargetKind{TargetSymbol, TargetSpan}

// specs contains one spec for every operation that [Operations] returns.
var specs = map[Operation]Spec{
	RenameSymbol: {
		Operation: RenameSymbol, Accepts: pointed,
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
		Operation: MoveSymbol, Accepts: pointed,
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
		Operation: ExtractInterface, Accepts: pointed,
		Required: []ArgKey{ArgNewName}, MinFidelity: trust.Resolved,
	},
	InlineVariable: {
		Operation: InlineVariable, Accepts: pointed,
		MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	InlineConstant: {
		Operation: InlineConstant, Accepts: pointed,
		MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	ChangeSignature: {
		Operation: ChangeSignature, Accepts: pointed,
		Required: []ArgKey{ArgSignature}, MinFidelity: trust.Resolved, RewritesReferences: true,
	},
	ImplementInterface: {
		Operation: ImplementInterface, Accepts: pointed,
		Optional: []ArgKey{ArgReceiver}, MinFidelity: trust.Resolved,
	},
	// DocumentSymbol inserts a comment above one declaration and changes
	// nothing else, so a parser serves it correctly. It is the only
	// operation with a Syntactic minimum.
	DocumentSymbol: {
		Operation: DocumentSymbol, Accepts: pointed,
		Required: []ArgKey{ArgDoc}, MinFidelity: trust.Syntactic,
	},
}

// SpecFor returns the spec of o and reports whether o is declared.
func SpecFor(o Operation) (Spec, bool) {
	s, ok := specs[o]
	return s, ok
}
