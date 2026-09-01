// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/techne/core/trust"
)

func TestEvidence(t *testing.T) {
	t.Parallel()

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("rises from none to resolved", func(t *testing.T) {
			t.Parallel()
			ordered := []trust.Fidelity{
				trust.None, trust.Syntactic, trust.Indexed, trust.Resolved,
			}
			for i := 1; i < len(ordered); i++ {
				if ordered[i-1] >= ordered[i] {
					t.Errorf("fidelity %d is not below %d", ordered[i-1], ordered[i])
				}
			}
		})

		t.Run("zero value is none", func(t *testing.T) {
			t.Parallel()
			var unset trust.Fidelity
			if unset != trust.None {
				t.Errorf("zero Fidelity = %d, want None", unset)
			}
		})
	})

	t.Run("Completeness", func(t *testing.T) {
		t.Parallel()

		t.Run("zero value claims nothing", func(t *testing.T) {
			t.Parallel()
			var unset trust.Completeness
			if unset != trust.ScopeUnknown {
				t.Errorf("zero Completeness = %d, want ScopeUnknown", unset)
			}
		})
	})

	t.Run("SupportsNegativeClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("resolved binding over total coverage licenses it", func(t *testing.T) {
			t.Parallel()
			if !trust.SupportsNegativeClaim(trust.Resolved, trust.ScopeTotal) {
				t.Error("resolved and total must license a negative claim")
			}
		})

		t.Run("resolved binding over partial coverage does not", func(t *testing.T) {
			t.Parallel()
			// A language server that binds through types while still
			// building its index. Reading its empty answer as
			// authoritative is the failure the second axis prevents.
			if trust.SupportsNegativeClaim(trust.Resolved, trust.ScopePartial) {
				t.Error("resolved but partial must not license a negative claim")
			}
		})

		t.Run("total coverage below resolved binding does not", func(t *testing.T) {
			t.Parallel()
			// A parser reads every file in scope and still matches on
			// name coincidence across files.
			if trust.SupportsNegativeClaim(trust.Syntactic, trust.ScopeTotal) {
				t.Error("syntactic and total must not license a negative claim")
			}
		})

		t.Run("no other pair licenses it", func(t *testing.T) {
			t.Parallel()
			for _, f := range []trust.Fidelity{trust.None, trust.Syntactic, trust.Indexed, trust.Resolved} {
				for _, c := range []trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal} {
					want := f == trust.Resolved && c == trust.ScopeTotal
					if got := trust.SupportsNegativeClaim(f, c); got != want {
						t.Errorf("SupportsNegativeClaim(%d, %d) = %v, want %v", f, c, got, want)
					}
				}
			}
		})
	})
}
