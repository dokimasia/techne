// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package core_test

import (
	"testing"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// TestDoc covers the claim the package comment makes for the module as a
// whole: these packages compose into an answer without any of them
// knowing a language.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the vocabulary composes", func(t *testing.T) {
		t.Parallel()

		t.Run("into an answer that states its own evidence", func(t *testing.T) {
			t.Parallel()
			id := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindType)
			answered := engine.Answer[sema.Symbol]{
				Items: []sema.Symbol{{
					ID:       id,
					Name:     "Status",
					Kind:     sema.KindType,
					Language: source.Language("go"),
					Span:     source.Span{Path: "core/trust/status.go"},
					Exported: true,
				}},
				Status: trust.OK,
				Provenance: trust.Provenance{
					Engine:       "gotypes",
					Fidelity:     trust.Resolved,
					Completeness: trust.ScopeTotal,
					Caveats:      []trust.Caveat{{Code: trust.CaveatDynamic}},
				},
			}

			if !answered.Status.Answered() {
				t.Error("an answer that ran must report a payload")
			}
			if !answered.Provenance.SupportsNegativeClaim() {
				t.Error("resolved binding over total coverage must license a negative claim")
			}
		})

		t.Run("into an empty answer nobody may read as proof", func(t *testing.T) {
			t.Parallel()
			// The case the two axes exist for: a server that binds
			// through types and has not finished indexing.
			warming := engine.Answer[sema.Symbol]{
				Status: trust.Partial,
				Provenance: trust.Provenance{
					Engine:       "lsp",
					Fidelity:     trust.Resolved,
					Completeness: trust.ScopePartial,
					Caveats:      []trust.Caveat{{Code: trust.CaveatIndexWarming}},
				},
			}

			if !warming.Status.Answered() {
				t.Error("a partial answer still ran and carries a payload worth reading")
			}
			if warming.Provenance.SupportsNegativeClaim() {
				t.Error("a warming index must not license a negative claim")
			}
			if len(warming.Items) != 0 {
				t.Error("this case is about an empty item list")
			}
		})
	})
}
