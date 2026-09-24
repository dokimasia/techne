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

func TestResult(t *testing.T) {
	t.Parallel()

	parser := fake{name: "parser", fidelity: trust.Syntactic, cost: engine.CostParse}
	checker := fake{name: "checker", fidelity: trust.Resolved, cost: engine.CostAnalyze}
	total := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}

	t.Run("Publish", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the engine name from the engine", func(t *testing.T) {
			t.Parallel()
			got := engine.Publish(total, checker, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Engine, "checker", "engine")
		})

		t.Run("takes the tier from the engine", func(t *testing.T) {
			t.Parallel()
			got := engine.Publish(total, checker, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Fidelity, trust.Resolved, "fidelity")
		})

		t.Run("lowers the tier to Lowered", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal, Lowered: trust.Indexed}
			got := engine.Publish(r, checker, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Fidelity, trust.Indexed, "fidelity")
		})

		t.Run("ignores a Lowered tier above the declared tier", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal, Lowered: trust.Resolved}
			got := engine.Publish(r, parser, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic, "fidelity")
		})

		t.Run("copies the completeness of the result", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Completeness: trust.ScopePartial}
			got := engine.Publish(r, checker, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Completeness, trust.ScopePartial, "completeness")
		})

		t.Run("copies the items of the result", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Items: []sema.Symbol{{Name: "F"}}}
			got := engine.Publish(r, checker, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Items, r.Items, "items")
		})

		t.Run("copies the caveats of the result", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Caveats: []trust.Caveat{{Code: trust.CaveatDynamic}}}
			got := engine.Publish(r, checker, engine.RoleOutline, trust.None)
			assert.Equal(t, got.Provenance.Caveats, r.Caveats, "caveats")
		})

		t.Run("copies Skipped from the result", func(t *testing.T) {
			t.Parallel()
			r := engine.Result[sema.Symbol]{Skipped: true}
			got := engine.Publish(r, checker, engine.RoleOutline, trust.None)
			assert.True(t, got.Skipped, "Skipped")
		})

		tests := []struct {
			name         string
			giveEngine   fake
			giveCoverage trust.Completeness
			giveWant     trust.Fidelity
			wantStatus   trust.Status
		}{
			{
				name:         "returns OK for total coverage at the requested tier",
				giveEngine:   checker,
				giveCoverage: trust.ScopeTotal,
				giveWant:     trust.Resolved,
				wantStatus:   trust.OK,
			},
			{
				name:         "returns Degraded below the requested tier",
				giveEngine:   parser,
				giveCoverage: trust.ScopeTotal,
				giveWant:     trust.Resolved,
				wantStatus:   trust.Degraded,
			},
			{
				name:         "returns Partial for partial coverage",
				giveEngine:   checker,
				giveCoverage: trust.ScopePartial,
				giveWant:     trust.Resolved,
				wantStatus:   trust.Partial,
			},
			{
				name:         "prefers Degraded over Partial",
				giveEngine:   parser,
				giveCoverage: trust.ScopePartial,
				giveWant:     trust.Resolved,
				wantStatus:   trust.Degraded,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				r := engine.Result[sema.Symbol]{Completeness: tt.giveCoverage}
				got := engine.Publish(r, tt.giveEngine, engine.RoleOutline, tt.giveWant)
				assert.Equal(t, got.Status, tt.wantStatus, "status")
			})
		}

		t.Run("returns an answered status for every completeness at every tier", func(t *testing.T) {
			t.Parallel()
			for _, coverage := range trust.Completenesses() {
				for _, want := range trust.Fidelities() {
					r := engine.Result[sema.Symbol]{Completeness: coverage}
					got := engine.Publish(r, parser, engine.RoleOutline, want)
					assert.True(t, got.Status.Answered(), coverage.String()+" at "+want.String())
				}
			}
		})
	})
}
