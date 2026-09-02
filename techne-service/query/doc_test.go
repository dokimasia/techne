// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/query"
)

// TestDoc covers the claim the package comment makes: nothing to ask is
// an answer, and a fault is not.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("a capability gap", func(t *testing.T) {
		t.Parallel()

		t.Run("is routed around, not raised", func(t *testing.T) {
			t.Parallel()
			// A caller can pick a different tool when told a language is
			// not served. It can do nothing with an error.
			got, err := query.New(engine.NewCatalog(), router{}).
				Outline(t.Context(), engine.Request{Scope: "a.fx", Language: fixture})

			assert.NoError(t, err, "having nothing to ask is not a fault")
			assert.Equal(t, got.Status, trust.Unsupported, "the caller is told the gap exists")
			assert.False(t, got.Provenance.SupportsNegativeClaim(),
				"an answer nothing produced proves nothing about what is there")
		})
	})
}
