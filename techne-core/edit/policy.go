// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit

import (
	"errors"
	"fmt"

	"go.dokimi.dev/techne/core/source"
)

// Errors returned by Policy.Admit. Callers branch on them with errors.Is.
var (
	// ErrWrongOperation means the plan was checked against the spec of
	// another operation.
	ErrWrongOperation = errors.New("edit: plan is for another operation")
	// ErrWeakEvidence means the plan's fidelity is below the operation's
	// minimum.
	ErrWeakEvidence = errors.New("edit: evidence is weaker than the operation requires")
	// ErrNoNegativeClaim means the operation rewrites references and the
	// plan's evidence does not rule out references it missed.
	ErrNoNegativeClaim = errors.New("edit: evidence does not rule out other references")
	// ErrDisordered means the edits of a change overlap or are out of order.
	ErrDisordered = errors.New("edit: edits overlap or are out of order")
	// ErrConflict means two changes of a plan contradict each other on one
	// path.
	ErrConflict = errors.New("edit: changes conflict on one path")
	// ErrUnsealed means a path the plan reads has no precondition.
	ErrUnsealed = errors.New("edit: path has no precondition")
	// ErrMalformed means a change lacks the field its kind requires.
	ErrMalformed = errors.New("edit: change is incomplete")
)

// Policy decides whether a plan may be applied. It is stateless and reads
// only the plan and the spec, so the same rules apply to every language.
type Policy struct{}

// Admit returns the first of these errors that applies to plan, checked in
// this order, or nil if none applies:
//
//   - ErrWrongOperation if the plan is for another operation.
//   - ErrWeakEvidence if the plan's fidelity is below spec.MinFidelity.
//   - ErrNoNegativeClaim if spec.RewritesReferences and the provenance does
//     not support a negative claim.
//   - ErrMalformed or ErrDisordered if a change is incomplete or its edits
//     are out of the order Apply requires.
//   - ErrConflict if two changes contradict each other on one path.
//   - ErrUnsealed if a path the plan reads has no precondition.
//
// Changes conflict when they break one of these rules:
//
//   - Each path has at most one edit, creation or deletion.
//   - A moved path can also be edited, and the edit applies before the move.
//   - Each path moves to at most one destination. A repeated move counts
//     once.
//   - A move destination has no other change. No second move targets it,
//     and it does not move again.
func (Policy) Admit(spec Spec, plan Plan) error {
	if plan.Operation != spec.Operation {
		return fmt.Errorf("%w: plan is %q, spec is %q", ErrWrongOperation, plan.Operation, spec.Operation)
	}
	if plan.Provenance.Fidelity < spec.MinFidelity {
		return fmt.Errorf("%w: %s requires %s, the plan has %s",
			ErrWeakEvidence, spec.Operation, spec.MinFidelity, plan.Provenance.Fidelity)
	}
	if spec.RewritesReferences && !plan.Provenance.SupportsNegativeClaim() {
		return fmt.Errorf("%w: the plan has %s evidence over %s coverage",
			ErrNoNegativeClaim, plan.Provenance.Fidelity, plan.Provenance.Completeness)
	}
	for _, c := range plan.Changes {
		if err := wellFormed(c); err != nil {
			return err
		}
	}
	if err := consistent(plan); err != nil {
		return err
	}
	return sealed(plan)
}

// wellFormed returns an error if c lacks the field its kind requires or its
// edits are out of order.
func wellFormed(c Change) error {
	switch c.Kind {
	case ChangeUnset:
		return fmt.Errorf("%w: change to %s has no kind", ErrMalformed, c.Path)
	case ChangeMove:
		switch c.To {
		case "":
			return fmt.Errorf("%w: move of %s has no destination", ErrMalformed, c.Path)
		case c.Path:
			return fmt.Errorf("%w: move of %s has itself as destination", ErrMalformed, c.Path)
		}
	case ChangeEdit:
		if len(c.Edits) == 0 {
			return fmt.Errorf("%w: edit of %s has no edits", ErrMalformed, c.Path)
		}
	case ChangeCreate, ChangeDelete:
	}
	if err := ordered(c.Edits); err != nil {
		return fmt.Errorf("%s: %w", c.Path, err)
	}
	return nil
}

// consistent returns an error wrapping ErrConflict if the changes of plan
// break a rule that Admit lists. It reports the first conflict in plan
// order.
func consistent(plan Plan) error {
	content := map[source.Path]ChangeKind{}
	for _, c := range plan.Changes {
		if c.Kind == ChangeMove {
			continue
		}
		if _, ok := content[c.Path]; ok {
			return fmt.Errorf("%w: %s is changed twice", ErrConflict, c.Path)
		}
		content[c.Path] = c.Kind
	}

	to := map[source.Path]source.Path{}
	from := map[source.Path]source.Path{}
	for _, c := range plan.Changes {
		if c.Kind != ChangeMove {
			continue
		}
		kind, changed := content[c.Path]
		_, overwritten := content[c.To]
		switch {
		case changed && kind != ChangeEdit:
			return fmt.Errorf("%w: %s is moved and also created or deleted", ErrConflict, c.Path)
		case overwritten:
			return fmt.Errorf("%w: %s is a move destination and also changed", ErrConflict, c.To)
		case to[c.Path] != "" && to[c.Path] != c.To:
			return fmt.Errorf("%w: %s is moved to both %s and %s", ErrConflict, c.Path, to[c.Path], c.To)
		case from[c.To] != "" && from[c.To] != c.Path:
			return fmt.Errorf("%w: %s is the destination of %s and of %s", ErrConflict, c.To, from[c.To], c.Path)
		}
		to[c.Path], from[c.To] = c.To, c.Path
	}
	for _, c := range plan.Changes {
		if _, again := to[c.To]; c.Kind == ChangeMove && again {
			return fmt.Errorf("%w: %s is moved to %s, which is moved again", ErrConflict, c.Path, c.To)
		}
	}
	return nil
}

// sealed returns an error wrapping ErrUnsealed if a path the plan reads has
// no precondition.
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
