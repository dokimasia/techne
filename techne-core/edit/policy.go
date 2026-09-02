// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"errors"
	"fmt"

	"go.dokimi.dev/techne/core/source"
)

// The reasons a plan is not applied. A caller branches on these rather
// than on the message: a rename refused for want of coverage is
// something waiting for an index would fix, and a plan whose edits
// overlap never will be.
var (
	// ErrWrongOperation reports a plan checked against another
	// operation's spec.
	ErrWrongOperation = errors.New("edit: the plan is for another operation")
	// ErrWeakEvidence reports evidence below the operation's minimum.
	ErrWeakEvidence = errors.New("edit: the evidence is weaker than this operation needs")
	// ErrNoNegativeClaim reports evidence that cannot support "there are
	// no other references".
	ErrNoNegativeClaim = errors.New("edit: rewriting references needs evidence that no others exist")
	// ErrDisordered reports edits that overlap or are out of order.
	ErrDisordered = errors.New("edit: the edits overlap or are out of order")
	// ErrUnsealed reports a path the plan touches with no precondition.
	ErrUnsealed = errors.New("edit: a path the plan touches was not pinned")
	// ErrMalformed reports a change carrying nothing to apply.
	ErrMalformed = errors.New("edit: the change carries nothing to apply")
)

// Policy decides whether a plan may be applied.
//
// It holds no state and reads nothing outside the plan and the spec, so
// the rule is reviewable in one place. Leaving it to each planner would
// put the judgement in the component with the strongest reason to be
// optimistic about itself.
type Policy struct{}

// Admit reports why a plan may not be applied, or nil.
//
// The checks run in a fixed order and the first failure is returned, so
// a caller told the evidence is too weak is not also told which edit
// overlapped: fixing the first would not reach the second.
func (Policy) Admit(spec Spec, plan Plan) error {
	if plan.Operation != spec.Operation {
		return fmt.Errorf("%w: plan is %q, spec is %q",
			ErrWrongOperation, plan.Operation, spec.Operation)
	}
	if plan.Provenance.Fidelity < spec.MinFidelity {
		return fmt.Errorf("%w: %s needs %s, the plan has %s",
			ErrWeakEvidence, spec.Operation, spec.MinFidelity, plan.Provenance.Fidelity)
	}
	// Finding every reference is a claim that no others exist. A server
	// that binds through types but has indexed half the workspace
	// reports resolved truthfully and would rewrite half the callers.
	if spec.RewritesReferences && !plan.Provenance.SupportsNegativeClaim() {
		return fmt.Errorf("%w: the plan covers %s of the scope at %s",
			ErrNoNegativeClaim, plan.Provenance.Completeness, plan.Provenance.Fidelity)
	}
	for _, c := range plan.Changes {
		if err := wellFormed(c); err != nil {
			return err
		}
	}
	return sealed(plan)
}

// wellFormed reports why one change cannot be applied in a single pass.
//
// The write path applies an edit list by walking it once, so it relies
// on the order and on the ranges not meeting. A planner that produced
// them in any other order would silently write the wrong bytes.
func wellFormed(c Change) error {
	switch c.Kind {
	case ChangeUnset:
		return fmt.Errorf("%w: %s names no kind", ErrMalformed, c.Path)
	case ChangeMove:
		if c.To == "" {
			return fmt.Errorf("%w: the move of %s names no destination", ErrMalformed, c.Path)
		}
	case ChangeEdit:
		if len(c.Edits) == 0 {
			return fmt.Errorf("%w: the edit of %s changes no range", ErrMalformed, c.Path)
		}
	case ChangeCreate, ChangeDelete:
	}

	end := 0
	for i, e := range c.Edits {
		start := e.Span.Start.Offset
		if start > e.Span.End.Offset {
			return fmt.Errorf("%w: %s edit %d ends before it starts", ErrDisordered, c.Path, i)
		}
		// An insertion is an empty range, so two of them at one point
		// would each be "not before" the other. Requiring the second to
		// start strictly later refuses that: which of the two lands
		// first is not something the plan says.
		if i > 0 && start <= end {
			return fmt.Errorf("%w: %s edit %d starts at %d, inside the edit ending at %d",
				ErrDisordered, c.Path, i, start, end)
		}
		end = e.Span.End.Offset
	}
	return nil
}

// sealed reports whether every path whose content the plan depends on
// was pinned.
func sealed(plan Plan) error {
	pinned := make(map[source.Path]bool, len(plan.Preconditions))
	for _, p := range plan.Preconditions {
		pinned[p.Path] = true
	}
	for _, p := range plan.Reads() {
		if !pinned[p] {
			return fmt.Errorf("%w: %s", ErrUnsealed, p)
		}
	}
	return nil
}
