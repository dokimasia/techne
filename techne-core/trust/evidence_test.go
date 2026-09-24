// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

func TestEvidence(t *testing.T) {
	t.Parallel()

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is None when zero", func(t *testing.T) {
			t.Parallel()
			var zero trust.Fidelity
			assert.Equal(t, zero, trust.None, "zero value")
		})

		t.Run("orders None below Syntactic below Indexed below Resolved", func(t *testing.T) {
			t.Parallel()
			assert.Pairwise(t, trust.Fidelities(), func(weaker, stronger trust.Fidelity) bool {
				return weaker < stronger
			}, "fidelity order")
		})
	})

	t.Run("Completeness", func(t *testing.T) {
		t.Parallel()

		t.Run("is ScopeUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero trust.Completeness
			assert.Equal(t, zero, trust.ScopeUnknown, "zero value")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pinned string of every fidelity", func(t *testing.T) {
			t.Parallel()
			for f, want := range map[trust.Fidelity]string{
				trust.None: "none", trust.Syntactic: "syntactic",
				trust.Indexed: "indexed", trust.Resolved: "resolved",
			} {
				assert.Equal(t, f.String(), want, "wire string")
			}
		})

		t.Run("returns none for an undeclared fidelity", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, trust.Fidelity(200).String(), "none", "wire string")
		})

		t.Run("returns the pinned string of every completeness", func(t *testing.T) {
			t.Parallel()
			for c, want := range map[trust.Completeness]string{
				trust.ScopeUnknown: "unknown", trust.ScopePartial: "partial", trust.ScopeTotal: "total",
			} {
				assert.Equal(t, c.String(), want, "wire string")
			}
		})

		t.Run("returns unknown for an undeclared completeness", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, trust.Completeness(200).String(), "unknown", "wire string")
		})
	})

	t.Run("Fidelities", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every fidelity once", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, trust.Fidelities(),
				[]trust.Fidelity{trust.None, trust.Syntactic, trust.Indexed, trust.Resolved}, "fidelities")
		})
	})

	t.Run("Completenesses", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every completeness once", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, trust.Completenesses(),
				[]trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal}, "completenesses")
		})
	})

	t.Run("SupportsNegativeClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true only for Resolved over ScopeTotal", func(t *testing.T) {
			t.Parallel()
			for _, f := range trust.Fidelities() {
				for _, c := range trust.Completenesses() {
					want := f == trust.Resolved && c == trust.ScopeTotal
					assert.Equal(t, trust.SupportsNegativeClaim(f, c), want, f.String()+"/"+c.String())
				}
			}
		})
	})
}
