// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

// TestDoc covers the contract the package comment states across both
// axes, which no single declaration owns.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("empty is never ambiguous", func(t *testing.T) {
		t.Parallel()

		t.Run("an answer that did not run never licenses a negative claim", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.Unset, trust.Unsupported, trust.Refused} {
				assert.False(t, s.Answered(),
					"a status carrying no payload cannot combine with any evidence to prove absence")
			}
			assert.True(t, trust.SupportsNegativeClaim(trust.Resolved, trust.ScopeTotal),
				"the strongest evidence still proves absence when an engine did answer")
		})
	})
}
