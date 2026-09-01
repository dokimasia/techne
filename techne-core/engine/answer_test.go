// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

func TestAnswer(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("claims nothing ran and nothing is proven", func(t *testing.T) {
			t.Parallel()
			// An answer a service forgot to fill must not read as a
			// successful empty result.
			var unset engine.Answer[sema.Symbol]
			assert.False(t, unset.Status.Answered(),
				"an answer a service forgot to fill must not read as a successful empty result")
			assert.False(t, unset.Provenance.SupportsNegativeClaim(),
				"an answer a service forgot to fill must not prove absence")
		})
	})

	t.Run("Items", func(t *testing.T) {
		t.Parallel()

		t.Run("being empty means nothing without the provenance", func(t *testing.T) {
			t.Parallel()
			// The same empty list under two provenances is two different
			// facts, which is why a caller reads both.
			proven := engine.Answer[sema.Symbol]{
				Status:     trust.OK,
				Provenance: trust.Provenance{Fidelity: trust.Resolved, Completeness: trust.ScopeTotal},
			}
			guessed := engine.Answer[sema.Symbol]{
				Status:     trust.OK,
				Provenance: trust.Provenance{Fidelity: trust.Syntactic, Completeness: trust.ScopeTotal},
			}
			assert.Equal(t, len(proven.Items), len(guessed.Items), "this case compares two empty answers")
			assert.True(t, proven.Provenance.SupportsNegativeClaim(),
				"one empty list proves there are none")
			assert.False(t, guessed.Provenance.SupportsNegativeClaim(),
				"the same empty list from a parser proves only that none were found")
		})
	})

	t.Run("Request", func(t *testing.T) {
		t.Parallel()

		t.Run("names the weakest evidence the caller will take", func(t *testing.T) {
			t.Parallel()
			// Preferred is a floor for reporting, not a filter: an engine
			// below it answers and is marked degraded rather than
			// refused.
			req := engine.Request{Preferred: trust.Resolved}
			assert.Equal(t, req.Preferred, trust.Resolved,
				"Preferred is a floor for reporting, so an engine below it answers and is marked degraded")
		})
	})

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		t.Run("matches any kind when none is named", func(t *testing.T) {
			t.Parallel()
			var q engine.Query
			assert.Equal(t, q.Kind, sema.KindUnknown, "a query naming no kind matches any")
			assert.Equal(t, q.Limit, 0, "a query naming no limit lets the engine choose")
		})
	})
}
