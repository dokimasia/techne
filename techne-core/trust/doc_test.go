// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Answered", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for every status without a payload", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.Unset, trust.Unsupported, trust.Refused} {
				assert.False(t, s.Answered(), s.String())
			}
		})
	})
}
