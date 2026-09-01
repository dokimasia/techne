// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

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
			if unset.Status.Answered() {
				t.Error("the zero Answer must not report a payload")
			}
			if unset.Provenance.SupportsNegativeClaim() {
				t.Error("the zero Answer must not license a negative claim")
			}
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
			if len(proven.Items) != len(guessed.Items) {
				t.Fatal("this case needs both answers empty")
			}
			if !proven.Provenance.SupportsNegativeClaim() {
				t.Error("a resolved answer over a total scope proves absence")
			}
			if guessed.Provenance.SupportsNegativeClaim() {
				t.Error("a parser's answer must not prove absence")
			}
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
			if req.Preferred != trust.Resolved {
				t.Errorf("Request.Preferred = %d, want Resolved", req.Preferred)
			}
		})
	})

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		t.Run("matches any kind when none is named", func(t *testing.T) {
			t.Parallel()
			var q engine.Query
			if q.Kind != sema.KindUnknown {
				t.Errorf("zero Query.Kind = %d, want KindUnknown so it matches any", q.Kind)
			}
			if q.Limit != 0 {
				t.Errorf("zero Query.Limit = %d, want 0 so the engine picks", q.Limit)
			}
		})
	})
}
