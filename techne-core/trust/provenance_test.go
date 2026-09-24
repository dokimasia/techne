// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

func TestProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Provenance", func(t *testing.T) {
		t.Parallel()

		t.Run("names no engine when zero", func(t *testing.T) {
			t.Parallel()
			var zero trust.Provenance
			assert.Empty(t, zero.Engine, "engine")
		})
	})

	t.Run("SupportsNegativeClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("agrees with the package function for every pair", func(t *testing.T) {
			t.Parallel()
			for _, f := range trust.Fidelities() {
				for _, c := range trust.Completenesses() {
					p := trust.Provenance{Fidelity: f, Completeness: c}
					assert.Equal(t, p.SupportsNegativeClaim(), trust.SupportsNegativeClaim(f, c),
						f.String()+"/"+c.String())
				}
			}
		})

		t.Run("returns false when zero", func(t *testing.T) {
			t.Parallel()
			var zero trust.Provenance
			assert.False(t, zero.SupportsNegativeClaim(), "negative claim")
		})
	})
}
