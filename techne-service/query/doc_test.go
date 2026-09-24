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

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an unsupported answer without an error for a language without an engine", func(t *testing.T) {
			t.Parallel()
			got, err := query.New(engine.NewCatalog(), router{}).
				Outline(t.Context(), engine.Request{Scope: "a.fx", Language: fixture})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the answer")
			assert.False(t, got.Provenance.SupportsNegativeClaim(), "the negative claim of the answer")
		})
	})
}
