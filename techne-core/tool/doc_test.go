// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/tool"
)

// TestDoc covers the claims the package comment makes about an answer
// that has already been thinned.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("fitting an answer twice", func(t *testing.T) {
		t.Parallel()

		t.Run("changes nothing the second time", func(t *testing.T) {
			t.Parallel()
			// A presenter that fits an answer a service already fitted
			// must not thin it further or claim a second truncation.
			budget := tool.Budget{MaxTokens: 300}
			once := tool.Fit(many(400), budget)
			twice := tool.Fit(once, budget)

			assert.Equal(t, twice.Items, once.Items, "an answer already inside the budget is left alone")
			assert.Equal(t, len(twice.Provenance.Caveats), len(once.Provenance.Caveats),
				"a second pass does not claim a second truncation")
		})
	})

	t.Run("thinning", func(t *testing.T) {
		t.Parallel()

		t.Run("never changes what a caller may conclude", func(t *testing.T) {
			t.Parallel()
			full := many(400)
			got := tool.Fit(full, tool.Budget{MaxTokens: 300})

			assert.Equal(t, got.Failed(), full.Failed(),
				"dropping items does not change whether an engine ran")
			assert.Equal(t, got.Provenance.Completeness, full.Provenance.Completeness,
				"what the engine covered is a fact about the engine, not about what fitted")
			assert.Equal(t, got.Provenance.SupportsNegativeClaim, full.Provenance.SupportsNegativeClaim,
				"thinning an answer says nothing about the evidence behind it")
		})
	})
}
