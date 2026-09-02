// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("rename.symbol", func(t *testing.T) {
		t.Parallel()

		t.Run("moves the declaration and every use of it", func(t *testing.T) {
			t.Parallel()
			// The whole point. A rename that moved nine of ten would
			// leave code that still parses and means something else,
			// which is the failure the policy exists to prevent and
			// which nothing could plan until now.
			got := planned(t, edit.RenameSymbol, pointing(t, "Store"),
				edit.Args{edit.ArgNewName: "Vault"})

			assert.Length(t, got.Items, 2, "the declaration's file and the one referring to it")
			assert.Equal(t, sites(got.Items), 4, "one declaration and three uses")
			for _, one := range got.Items {
				for _, e := range one.Edits {
					assert.Equal(t, e.New, "Vault", "every site becomes the new name")
				}
			}
		})

		t.Run("touches the name and not the line around it", func(t *testing.T) {
			t.Parallel()
			// Rewriting the line would rewrite the keyword too, which is
			// the difference between a rename and a substitution.
			got := planned(t, edit.RenameSymbol, pointing(t, "Store"),
				edit.Args{edit.ArgNewName: "Vault"})
			content := workspace()["src/store.mock"].Data

			for _, one := range got.Items {
				if one.Path != "src/store.mock" {
					continue
				}
				first := one.Edits[0].Span
				assert.Equal(t, string(content[first.Start.Offset:first.End.Offset]), "Store",
					"the span covers the name alone")
			}
		})

		t.Run("orders the edits, as the write path requires", func(t *testing.T) {
			t.Parallel()
			// The write path walks an edit list once and never looks
			// back, so an unsorted list writes the wrong bytes.
			got := planned(t, edit.RenameSymbol, pointing(t, "Store"),
				edit.Args{edit.ArgNewName: "Vault"})
			for _, one := range got.Items {
				for i := 1; i < len(one.Edits); i++ {
					assert.True(t, one.Edits[i-1].Span.End.Offset <= one.Edits[i].Span.Start.Offset,
						"edits run forward and do not meet")
				}
			}
		})

		t.Run("reports the coverage it was registered to claim", func(t *testing.T) {
			t.Parallel()
			// The policy reads it. An operation that rewrites references
			// is refused on partial coverage however strong the binding,
			// and a plan that did not carry it could not be refused.
			got := planned(t, edit.RenameSymbol, pointing(t, "Store"),
				edit.Args{edit.ArgNewName: "Vault"})
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"the whole workspace was read, so there is no other reference")
		})

		t.Run("refuses a rename to the name it already has", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol, pointing(t, "Store"), edit.Args{edit.ArgNewName: "Store"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "a rename that changes nothing is not one")
		})

		t.Run("refuses a target naming nothing", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "src/store.mock"}},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "there is nothing at that position")
		})
	})

	t.Run("document.symbol", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a comment above the declaration", func(t *testing.T) {
			t.Parallel()
			got := planned(t, edit.DocumentSymbol, pointing(t, "New"),
				edit.Args{edit.ArgDoc: "Make one."})
			assert.Length(t, got.Items, 1, "documenting one declaration changes one file")
			assert.Equal(t, got.Items[0].Edits[0].New, ";; Make one.\n",
				"in the form this language reads")
		})

		t.Run("replaces the documentation already there", func(t *testing.T) {
			t.Parallel()
			// A declaration carrying two comments is one this language's
			// reader takes only the second of.
			got := planned(t, edit.DocumentSymbol, pointing(t, "Store"),
				edit.Args{edit.ArgDoc: "Something else."})
			content := workspace()["src/store.mock"].Data
			at := got.Items[0].Edits[0].Span

			assert.Contains(t, string(content[at.Start.Offset:at.End.Offset]), "holds items",
				"what goes is the comment that was there")
		})

		t.Run("refuses a call with nothing to write", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.DocumentSymbol, pointing(t, "New"), edit.Args{edit.ArgDoc: "  "})
			assert.ErrorIs(t, err, engine.ErrRefuse, "there is nothing to write")
		})
	})

	t.Run("an operation this language has no planner for", func(t *testing.T) {
		t.Parallel()

		t.Run("is declined, so another engine gets a turn", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"},
				edit.ExtractFunction, pointing(t, "New"), edit.Args{edit.ArgNewName: "x"})
			assert.ErrorIs(t, err, engine.ErrDecline,
				"declining is not refusing: something else may serve it")
		})
	})
}

// planned runs the planner and fails the case if it will not.
func planned(
	t *testing.T,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) engine.Result[edit.Change] {
	t.Helper()
	got, err := built(t).Plan(t.Context(), engine.Request{Scope: "src"}, op, target, args)
	assert.NoError(t, err, "planning an operation this language serves succeeds")
	return got
}

// pointing is the target naming one declaration by where it is.
func pointing(t *testing.T, name string) edit.Target {
	t.Helper()
	got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src"})
	assert.NoError(t, err, "outlining succeeds")

	one, found := named(got.Items, name)
	assert.True(t, found, "the case names a declaration the workspace holds")
	return edit.Target{Kind: edit.TargetSpan, Span: one.Span}
}

// sites counts the ranges a plan rewrites.
func sites(changes []edit.Change) int {
	held := 0
	for _, one := range changes {
		held += len(one.Edits)
	}
	return held
}
