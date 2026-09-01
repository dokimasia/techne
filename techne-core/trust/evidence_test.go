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

		t.Run("rises from none to resolved", func(t *testing.T) {
			t.Parallel()
			ordered := []trust.Fidelity{
				trust.None, trust.Syntactic, trust.Indexed, trust.Resolved,
			}
			assert.Pairwise(t, ordered, func(earlier, later trust.Fidelity) bool {
				return earlier < later
			}, "each tier sorts below the next, because the catalogue orders engines by this")
		})

		t.Run("zero value is none", func(t *testing.T) {
			t.Parallel()
			var unset trust.Fidelity
			assert.Equal(t, unset, trust.None, "an unset fidelity claims nothing")
		})
	})

	t.Run("Completeness", func(t *testing.T) {
		t.Parallel()

		t.Run("zero value claims nothing", func(t *testing.T) {
			t.Parallel()
			var unset trust.Completeness
			assert.Equal(t, unset, trust.ScopeUnknown,
				"an engine that says nothing about its coverage has claimed nothing")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of a tier", func(t *testing.T) {
			t.Parallel()
			for f, want := range map[trust.Fidelity]string{
				trust.None: "none", trust.Syntactic: "syntactic",
				trust.Indexed: "indexed", trust.Resolved: "resolved",
			} {
				assert.Equal(t, f.String(), want, "an answer carries this string to a caller")
			}
			assert.Equal(t, trust.Fidelity(200).String(), "none",
				"a tier outside the set claims nothing rather than reading as an empty name")
		})

		t.Run("is the wire form of a coverage claim", func(t *testing.T) {
			t.Parallel()
			for c, want := range map[trust.Completeness]string{
				trust.ScopeUnknown: "unknown", trust.ScopePartial: "partial", trust.ScopeTotal: "total",
			} {
				assert.Equal(t, c.String(), want, "an answer carries this string to a caller")
			}
			assert.Equal(t, trust.Completeness(200).String(), "unknown",
				"a claim outside the set reads as unknown rather than as an empty name")
		})
	})

	t.Run("SupportsNegativeClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("resolved binding over total coverage licenses it", func(t *testing.T) {
			t.Parallel()
			assert.True(t, trust.SupportsNegativeClaim(trust.Resolved, trust.ScopeTotal),
				"a type checker that saw the whole scope proves an empty answer means there are none")
		})

		t.Run("resolved binding over partial coverage does not", func(t *testing.T) {
			t.Parallel()
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, trust.ScopePartial),
				"a server still building its index found some references, not all of them")
		})

		t.Run("total coverage below resolved binding does not", func(t *testing.T) {
			t.Parallel()
			assert.False(t, trust.SupportsNegativeClaim(trust.Syntactic, trust.ScopeTotal),
				"a parser reading every file still matches on name coincidence across files")
		})

		t.Run("no other pair licenses it", func(t *testing.T) {
			t.Parallel()
			for _, f := range []trust.Fidelity{trust.None, trust.Syntactic, trust.Indexed, trust.Resolved} {
				for _, c := range []trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal} {
					want := f == trust.Resolved && c == trust.ScopeTotal
					assert.Equal(t, trust.SupportsNegativeClaim(f, c), want,
						"only resolved binding together with total coverage licenses a negative claim")
				}
			}
		})
	})
}
