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

	document, _ := edit.SpecFor(edit.DocumentSymbol)
	rename, _ := edit.SpecFor(edit.RenameSymbol)

	t.Run("Admit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a plan that meets its spec", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, edit.Policy{}.Admit(document, documenting()), "Admit")
		})

		t.Run("returns ErrWrongOperation for a plan of another operation", func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, edit.Policy{}.Admit(rename, documenting()), edit.ErrWrongOperation, "Admit")
		})

		t.Run("returns ErrWeakEvidence below the minimum fidelity", func(t *testing.T) {
			t.Parallel()
			plan := documenting()
			plan.Operation = edit.RenameSymbol
			assert.ErrorIs(t, edit.Policy{}.Admit(rename, plan), edit.ErrWeakEvidence, "Admit")
		})

		t.Run("returns ErrNoNegativeClaim for a reference rewrite over partial coverage", func(t *testing.T) {
			t.Parallel()
			plan := documenting()
			plan.Operation = edit.RenameSymbol
			plan.Provenance = trust.Provenance{Fidelity: trust.Resolved, Completeness: trust.ScopePartial}
			assert.ErrorIs(t, edit.Policy{}.Admit(rename, plan), edit.ErrNoNegativeClaim, "Admit")
		})

		t.Run("returns nil for partial coverage of an operation without reference rewrites", func(t *testing.T) {
			t.Parallel()
			plan := documenting()
			plan.Provenance.Completeness = trust.ScopePartial
			assert.NoError(t, edit.Policy{}.Admit(document, plan), "Admit")
		})

		t.Run("returns ErrUnsealed for a path without a precondition", func(t *testing.T) {
			t.Parallel()
			plan := documenting()
			plan.Preconditions = nil
			assert.ErrorIs(t, edit.Policy{}.Admit(document, plan), edit.ErrUnsealed, "Admit")
		})

		orders := []struct {
			name string
			give []edit.TextEdit
			want error
		}{
			{
				name: "returns ErrDisordered for edits out of order",
				give: []edit.TextEdit{at(40, 50, "b"), at(10, 20, "a")},
				want: edit.ErrDisordered,
			},
			{
				name: "returns ErrDisordered for overlapping edits",
				give: []edit.TextEdit{at(10, 20, "a"), at(15, 25, "b")},
				want: edit.ErrDisordered,
			},
			{name: "returns nil for touching edits", give: []edit.TextEdit{at(10, 20, "a"), at(20, 25, "b")}},
			{name: "returns nil for inserts at one offset", give: []edit.TextEdit{at(10, 10, "a"), at(10, 10, "b")}},
		}
		for _, tt := range orders {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				changed := edit.Change{Kind: edit.ChangeEdit, Path: "a.go", Edits: tt.give}
				assertAdmitted(t, edit.Policy{}.Admit(document, planning(changed)), tt.want)
			})
		}

		malformed := []struct {
			name string
			give edit.Change
		}{
			{name: "returns ErrMalformed for a change without a kind", give: edit.Change{Path: "a.go"}},
			{
				name: "returns ErrMalformed for an edit without edits",
				give: edit.Change{Kind: edit.ChangeEdit, Path: "a.go"},
			},
			{
				name: "returns ErrMalformed for a move without a destination",
				give: edit.Change{Kind: edit.ChangeMove, Path: "a.go"},
			},
			{
				name: "returns ErrMalformed for a move onto its own path",
				give: edit.Change{Kind: edit.ChangeMove, Path: "a.go", To: "a.go"},
			},
		}
		for _, tt := range malformed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.ErrorIs(t, edit.Policy{}.Admit(document, planning(tt.give)), edit.ErrMalformed, "Admit")
			})
		}

		conflicts := []struct {
			name string
			give []edit.Change
			want error
		}{
			{
				name: "returns ErrConflict for two edits of one path",
				give: []edit.Change{editOf("a.go", 0), editOf("a.go", 5)},
				want: edit.ErrConflict,
			},
			{
				name: "returns nil for an edit of a moved path",
				give: []edit.Change{editOf("a.go", 0), move("a.go", "b.go")},
			},
			{
				name: "returns ErrConflict for a deletion of a moved path",
				give: []edit.Change{{Kind: edit.ChangeDelete, Path: "a.go"}, move("a.go", "b.go")},
				want: edit.ErrConflict,
			},
			{
				name: "returns ErrConflict for a creation at a move destination",
				give: []edit.Change{
					move("a.go", "b.go"),
					{Kind: edit.ChangeCreate, Path: "b.go", Content: []byte("x")},
				},
				want: edit.ErrConflict,
			},
			{
				name: "returns nil for a repeated move",
				give: []edit.Change{move("a.go", "b.go"), move("a.go", "b.go")},
			},
			{
				name: "returns ErrConflict for two destinations of one path",
				give: []edit.Change{move("a.go", "b.go"), move("a.go", "c.go")},
				want: edit.ErrConflict,
			},
			{
				name: "returns ErrConflict for two moves to one destination",
				give: []edit.Change{move("a.go", "c.go"), move("b.go", "c.go")},
				want: edit.ErrConflict,
			},
			{
				name: "returns ErrConflict for a destination that is moved again",
				give: []edit.Change{move("a.go", "b.go"), move("b.go", "c.go")},
				want: edit.ErrConflict,
			},
		}
		for _, tt := range conflicts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assertAdmitted(t, edit.Policy{}.Admit(document, planning(tt.give...)), tt.want)
			})
		}
	})
}

// assertAdmitted checks that err is nil when want is nil, and that err wraps
// want otherwise.
func assertAdmitted(t *testing.T, err, want error) {
	t.Helper()
	if want == nil {
		assert.NoError(t, err, "Admit")
		return
	}
	assert.ErrorIs(t, err, want, "Admit")
}

// documenting returns a document.symbol plan that Admit accepts.
func documenting() edit.Plan {
	return planning(edit.Change{Kind: edit.ChangeEdit, Path: "a.go", Edits: []edit.TextEdit{at(0, 0, "// Doc.\n")}})
}

// planning returns a sealed document.symbol plan over changes.
func planning(changes ...edit.Change) edit.Plan {
	plan := edit.Plan{
		Operation: edit.DocumentSymbol,
		Changes:   changes,
		Provenance: trust.Provenance{
			Engine: "test", Fidelity: trust.Syntactic, Completeness: trust.ScopeTotal,
		},
	}
	for _, p := range plan.Reads() {
		plan.Preconditions = append(plan.Preconditions, edit.Precondition{Path: p, Digest: sha256.Sum256(nil)})
	}
	return plan
}

// editOf returns an edit of p that replaces the byte at offset.
func editOf(p source.Path, offset int) edit.Change {
	return edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{at(offset, offset+1, "x")}}
}

// move returns a move of from to to.
func move(from, to source.Path) edit.Change {
	return edit.Change{Kind: edit.ChangeMove, Path: from, To: to}
}

// at returns an edit of the byte range [from, to).
func at(from, to int, text string) edit.TextEdit {
	return edit.TextEdit{
		Span: source.Span{Start: source.Position{Offset: from}, End: source.Position{Offset: to}},
		New:  text,
	}
}
