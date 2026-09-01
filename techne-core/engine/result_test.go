// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestResult(t *testing.T) {
	t.Parallel()

	const lang = source.Language("fixture")
	answered := fake{name: "parser", lang: lang, fidelity: trust.Syntactic, cost: engine.CostParse}
	strong := fake{name: "checker", lang: lang, fidelity: trust.Resolved, cost: engine.CostAnalyze}

	t.Run("Publish", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the engine's name and tier, not the result's word", func(t *testing.T) {
			t.Parallel()
			// A Result has no field for either. An adapter cannot claim
			// a tier it does not hold, which is the same reason a role
			// is declined by lacking a method.
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}
			got := engine.Publish(r, strong, engine.RoleOutline, trust.None)

			if got.Provenance.Engine != "checker" {
				t.Errorf("provenance names %q, want the engine that answered", got.Provenance.Engine)
			}
			if got.Provenance.Fidelity != trust.Resolved {
				t.Errorf("provenance tier = %d, want the engine's own", got.Provenance.Fidelity)
			}
		})

		t.Run("carries the completeness only the engine knows", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			got := engine.Publish(r, strong, engine.RoleOutline, trust.None)
			if got.Provenance.Completeness != trust.ScopePartial {
				t.Errorf("completeness = %d, want the engine's own", got.Provenance.Completeness)
			}
		})

		t.Run("reports OK when the tier was met and the scope covered", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}
			if got := engine.Publish(r, strong, engine.RoleOutline, trust.Resolved); got.Status != trust.OK {
				t.Errorf("status = %d, want OK", got.Status)
			}
		})

		t.Run("reports Degraded when the tier is below what was asked", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}
			if got := engine.Publish(r, answered, engine.RoleOutline, trust.Resolved); got.Status != trust.Degraded {
				t.Errorf("status = %d, want Degraded", got.Status)
			}
		})

		t.Run("reports Partial when the scope was not covered", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			if got := engine.Publish(r, strong, engine.RoleOutline, trust.Resolved); got.Status != trust.Partial {
				t.Errorf("status = %d, want Partial", got.Status)
			}
		})

		t.Run("prefers Degraded when the tier is short and the scope is too", func(t *testing.T) {
			t.Parallel()
			// Both are true and the status is one value. The caller
			// asked for a floor and did not get it, which changes what
			// the answer is worth more than a named gap does; the gap
			// is still in the caveats.
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			if got := engine.Publish(r, answered, engine.RoleOutline, trust.Resolved); got.Status != trust.Degraded {
				t.Errorf("status = %d, want Degraded", got.Status)
			}
		})

		t.Run("never reports Unset", func(t *testing.T) {
			t.Parallel()
			// A published answer has run. Leaving the zero status would
			// make it read as though nothing did.
			for _, c := range []trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal} {
				for _, want := range []trust.Fidelity{trust.None, trust.Syntactic, trust.Resolved} {
					r := engine.Result[sema.Symbol]{Completeness: c}
					got := engine.Publish(r, answered, engine.RoleOutline, want)
					if got.Status == trust.Unset {
						t.Errorf("completeness %d against floor %d published an Unset status", c, want)
					}
					if !got.Status.Answered() {
						t.Errorf("completeness %d against floor %d published a status carrying no payload", c, want)
					}
				}
			}
		})

		t.Run("carries the items and caveats through unchanged", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{
				Items:        []sema.Symbol{{Name: "F"}},
				Completeness: trust.ScopeTotal,
				Caveats:      []trust.Caveat{{Code: trust.CaveatDynamic}},
			}
			got := engine.Publish(r, strong, engine.RoleOutline, trust.None)
			if len(got.Items) != 1 || got.Items[0].Name != "F" {
				t.Errorf("items = %v, want the engine's own", got.Items)
			}
			if len(got.Provenance.Caveats) != 1 || got.Provenance.Caveats[0].Code != trust.CaveatDynamic {
				t.Errorf("caveats = %v, want the engine's own", got.Provenance.Caveats)
			}
		})
	})
}
