// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

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
			// Whatever evidence an engine would have advertised, a
			// status that produced no payload must not combine with it
			// to make an empty list look authoritative.
			for _, s := range []trust.Status{trust.Unset, trust.Unsupported, trust.Refused} {
				if s.Answered() {
					t.Fatalf("status %d reports a payload; the rest of this case assumes it does not", s)
				}
			}
			if !trust.SupportsNegativeClaim(trust.Resolved, trust.ScopeTotal) {
				t.Error("the strongest evidence must still license a negative claim when an engine did answer")
			}
		})
	})
}
