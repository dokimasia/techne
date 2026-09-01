// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestProvenance(t *testing.T) {
	t.Parallel()

	t.Run("SupportsNegativeClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("agrees with the package function", func(t *testing.T) {
			t.Parallel()
			for _, f := range []trust.Fidelity{trust.None, trust.Syntactic, trust.Indexed, trust.Resolved} {
				for _, c := range []trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal} {
					p := trust.Provenance{Fidelity: f, Completeness: c}
					assert.Equal(t, p.SupportsNegativeClaim(), trust.SupportsNegativeClaim(f, c),
						"the method delegates rather than repeating the rule, so the two cannot drift")
				}
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("licenses nothing", func(t *testing.T) {
			t.Parallel()
			var unset trust.Provenance
			assert.False(t, unset.SupportsNegativeClaim(),
				"an answer nobody stamped must not read as proof of absence")
			assert.Empty(t, unset.Engine, "an unstamped provenance names no engine")
		})
	})

	t.Run("Caveat", func(t *testing.T) {
		t.Parallel()

		t.Run("names the files it is about", func(t *testing.T) {
			t.Parallel()
			stale := trust.Caveat{
				Code:  trust.CaveatStale,
				Paths: []source.Path{"core/trust/status.go"},
			}
			assert.NotEmpty(t, stale.Paths,
				"a caller told which files drifted keeps the rest of the answer")
		})
	})
}
