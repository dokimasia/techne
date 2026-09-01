// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
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

			assert.Equal(t, got.Provenance.Engine, "checker",
				"a Result has no field for a name, so the engine that answered is named by Publish")
			assert.Equal(t, got.Provenance.Fidelity, trust.Resolved,
				"a Result has no field for a tier, so an adapter cannot claim one it does not hold")
		})

		t.Run("carries the completeness only the engine knows", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			got := engine.Publish(r, strong, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Completeness, trust.ScopePartial,
				"only the engine knows what it covered, so completeness comes from the result")
		})

		t.Run("reports OK when the tier was met and the scope covered", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}
			assert.Equal(t, engine.Publish(r, strong, engine.RoleOutline, trust.Resolved).Status, trust.OK,
				"the tier the caller asked for was met and the scope was covered")
		})

		t.Run("reports Degraded when the tier is below what was asked", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}
			assert.Equal(t, engine.Publish(r, answered, engine.RoleOutline, trust.Resolved).Status, trust.Degraded,
				"an engine below the caller's floor answers and says so rather than refusing")
		})

		t.Run("reports Partial when the scope was not covered", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			assert.Equal(t, engine.Publish(r, strong, engine.RoleOutline, trust.Resolved).Status, trust.Partial,
				"a scope the engine did not cover is reported, and the caveats name the gap")
		})

		t.Run("prefers Degraded when the tier is short and the scope is too", func(t *testing.T) {
			t.Parallel()
			// Both are true and the status is one value. The caller
			// asked for a floor and did not get it, which changes what
			// the answer is worth more than a named gap does; the gap
			// is still in the caveats.
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			assert.Equal(t, engine.Publish(r, answered, engine.RoleOutline, trust.Resolved).Status, trust.Degraded,
				"a caller that named a floor and missed it learns more from that than from a gap the caveats name")
		})

		t.Run("never reports Unset", func(t *testing.T) {
			t.Parallel()
			// A published answer has run. Leaving the zero status would
			// make it read as though nothing did.
			for _, c := range []trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal} {
				for _, want := range []trust.Fidelity{trust.None, trust.Syntactic, trust.Resolved} {
					r := engine.Result[sema.Symbol]{Completeness: c}
					got := engine.Publish(r, answered, engine.RoleOutline, want)
					assert.NotEqual(t, got.Status, trust.Unset,
						"a published answer has run, so it never reads as though nothing did")
					assert.True(t, got.Status.Answered(),
						"a published answer has run, so it always carries a payload")
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
			assert.Length(t, got.Items, 1, "stamping an answer does not change what the engine found")
			assert.Equal(t, got.Items[0].Name, "F", "stamping an answer does not change what the engine found")
			assert.Length(t, got.Provenance.Caveats, 1, "a limit only the engine knew about survives stamping")
			assert.Equal(t, got.Provenance.Caveats[0].Code, trust.CaveatDynamic,
				"a limit only the engine knew about survives stamping")
		})
	})
}
