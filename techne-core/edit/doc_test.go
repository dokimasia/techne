// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("SpecFor", func(t *testing.T) {
		t.Parallel()

		t.Run("requires total coverage for every reference rewrite", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				spec, _ := edit.SpecFor(op)
				if !spec.RewritesReferences {
					continue
				}
				assert.False(t, trust.SupportsNegativeClaim(spec.MinFidelity, trust.ScopePartial), string(op))
				assert.True(t, trust.SupportsNegativeClaim(spec.MinFidelity, trust.ScopeTotal), string(op))
			}
		})
	})
}
