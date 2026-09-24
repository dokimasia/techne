// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
)

func TestAnswer(t *testing.T) {
	t.Parallel()

	t.Run("Answer", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no payload when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Answer[sema.Symbol]
			assert.False(t, zero.Status.Answered(), "Answered")
		})

		t.Run("supports no negative claim when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Answer[sema.Symbol]
			assert.False(t, zero.Provenance.SupportsNegativeClaim(), "SupportsNegativeClaim")
		})
	})

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		t.Run("sets Kind to KindUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Query
			assert.Equal(t, zero.Kind, sema.KindUnknown, "Kind")
		})
	})
}
