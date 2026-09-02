// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"crypto/sha256"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Admit", func(t *testing.T) {
		t.Parallel()

		t.Run("admits a plan that meets what its operation declares", func(t *testing.T) {
			t.Parallel()
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			assert.NoError(t, edit.Policy{}.Admit(spec, documenting()),
				"a plan at the tier its operation declares, pinned and in order, is applied")
		})

		t.Run("refuses evidence weaker than the operation needs", func(t *testing.T) {
			t.Parallel()
			spec, _ := edit.SpecFor(edit.RenameSymbol)
			plan := documenting()
			plan.Operation = edit.RenameSymbol
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrWeakEvidence,
				"a rename computed from matched text moves the names that happened to match")
		})

		t.Run("refuses a reference rewrite that cannot claim there are no others", func(t *testing.T) {
			t.Parallel()
			// A server that binds through types and has indexed half the
			// workspace reports resolved truthfully. The tier alone
			// would admit it, and the rename would move half the callers.
			spec, _ := edit.SpecFor(edit.RenameSymbol)
			plan := documenting()
			plan.Operation = edit.RenameSymbol
			plan.Provenance = trust.Provenance{
				Fidelity: trust.Resolved, Completeness: trust.ScopePartial,
			}
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrNoNegativeClaim,
				"finding every reference is a claim that no others exist")
		})

		t.Run("admits a change that rewrites nothing else on the same evidence", func(t *testing.T) {
			t.Parallel()
			// The completeness check is tied to rewriting references
			// rather than to writing at all: a comment written above one
			// declaration says nothing about what is elsewhere.
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			plan := documenting()
			plan.Provenance.Completeness = trust.ScopePartial
			assert.NoError(t, edit.Policy{}.Admit(spec, plan),
				"an operation that changes one declaration makes no claim about the rest")
		})

		t.Run("refuses a plan checked against another operation's spec", func(t *testing.T) {
			t.Parallel()
			spec, _ := edit.SpecFor(edit.RenameSymbol)
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, documenting()), edit.ErrWrongOperation,
				"a plan admitted under the wrong spec is admitted under the wrong rules")
		})

		t.Run("refuses edits that are out of order", func(t *testing.T) {
			t.Parallel()
			// The write path walks an edit list once and never looks
			// back, so an unsorted list writes the wrong bytes.
			plan := documenting()
			plan.Changes[0].Edits = []edit.TextEdit{at(40, 50, "b"), at(10, 20, "a")}
			plan.Preconditions = pinned(plan)
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrDisordered,
				"a walk that copies what lies between edits relies on their order")
		})

		t.Run("refuses edits that meet", func(t *testing.T) {
			t.Parallel()
			plan := documenting()
			plan.Changes[0].Edits = []edit.TextEdit{at(10, 20, "a"), at(15, 25, "b")}
			plan.Preconditions = pinned(plan)
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrDisordered,
				"two edits over one range describe two different files")
		})

		t.Run("refuses two insertions at one point", func(t *testing.T) {
			t.Parallel()
			// Which of the two lands first is not something the plan
			// says, so the result depends on the order they happen to
			// be in.
			plan := documenting()
			plan.Changes[0].Edits = []edit.TextEdit{at(10, 10, "a"), at(10, 10, "b")}
			plan.Preconditions = pinned(plan)
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrDisordered,
				"an ordering the plan does not state is one the write path invents")
		})

		t.Run("refuses a path the plan touches and nobody pinned", func(t *testing.T) {
			t.Parallel()
			// Byte ranges computed against different content describe
			// something else, and the result usually still compiles.
			plan := documenting()
			plan.Preconditions = nil
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrUnsealed,
				"a plan nobody pinned applies cleanly to a file it never saw")
		})

		t.Run("refuses a change carrying nothing to apply", func(t *testing.T) {
			t.Parallel()
			spec, _ := edit.SpecFor(edit.DocumentSymbol)
			for _, c := range []edit.Change{
				{Path: "a.go"},
				{Kind: edit.ChangeEdit, Path: "a.go"},
				{Kind: edit.ChangeMove, Path: "a.go"},
			} {
				plan := documenting()
				plan.Changes = []edit.Change{c}
				plan.Preconditions = pinned(plan)
				assert.ErrorIs(t, edit.Policy{}.Admit(spec, plan), edit.ErrMalformed,
					"a change that says it does something and carries nothing is a bug, not a refusal")
			}
		})
	})
}

// documenting is a plan that meets everything document.symbol declares.
func documenting() edit.Plan {
	plan := edit.Plan{
		Operation: edit.DocumentSymbol,
		Changes: []edit.Change{{
			Kind: edit.ChangeEdit, Path: "a.go",
			Edits: []edit.TextEdit{at(0, 0, "// Doc.\n")},
		}},
		Provenance: trust.Provenance{
			Engine: "test", Fidelity: trust.Syntactic, Completeness: trust.ScopeTotal,
		},
	}
	plan.Preconditions = pinned(plan)
	return plan
}

// pinned seals every path a plan depends on, as the write path does.
func pinned(plan edit.Plan) []edit.Precondition {
	var out []edit.Precondition
	for _, p := range plan.Reads() {
		out = append(out, edit.Precondition{Path: p, Digest: sha256.Sum256(nil)})
	}
	return out
}

// at is one edit over a byte range.
func at(from, to int, text string) edit.TextEdit {
	return edit.TextEdit{
		Span: source.Span{
			Start: source.Position{Offset: from},
			End:   source.Position{Offset: to},
		},
		New: text,
	}
}
