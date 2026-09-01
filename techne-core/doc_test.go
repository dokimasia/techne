// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package core_test

import (
	"testing"

	"go.dokimi.dev/assert"
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

			assert.True(t, answered.Status.Answered(), "an engine ran and returned what it found")
			assert.True(t, answered.Provenance.SupportsNegativeClaim(),
				"a type checker that saw the whole scope proves an empty answer means there are none")
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

			assert.True(t, warming.Status.Answered(), "a partial answer still ran and is worth reading")
			assert.False(t, warming.Provenance.SupportsNegativeClaim(),
				"a server still building its index has not seen everything it would need to prove absence")
			assert.Empty(t, warming.Items, "this case is about what an empty item list is worth")
		})
	})
}
