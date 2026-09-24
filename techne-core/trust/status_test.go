// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/trust"
)

func TestStatus(t *testing.T) {
	t.Parallel()

	t.Run("Status", func(t *testing.T) {
		t.Parallel()

		t.Run("is Unset when zero", func(t *testing.T) {
			t.Parallel()
			var zero trust.Status
			assert.Equal(t, zero, trust.Unset, "zero value")
		})

		t.Run("keeps Unsupported distinct from Refused", func(t *testing.T) {
			t.Parallel()
			assert.NotEqual(t, trust.Unsupported, trust.Refused, "statuses")
		})
	})

	t.Run("Answered", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true for statuses with a payload", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.OK, trust.Degraded, trust.Partial} {
				assert.True(t, s.Answered(), s.String())
			}
		})

		t.Run("returns false for statuses without a payload", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.Unset, trust.Unsupported, trust.Refused} {
				assert.False(t, s.Answered(), s.String())
			}
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pinned string of every status", func(t *testing.T) {
			t.Parallel()
			for s, want := range map[trust.Status]string{
				trust.Unset: "unset", trust.OK: "ok", trust.Degraded: "degraded",
				trust.Partial: "partial", trust.Unsupported: "unsupported", trust.Refused: "refused",
			} {
				assert.Equal(t, s.String(), want, "wire string")
			}
		})

		t.Run("returns unset for an undeclared status", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, trust.Status(200).String(), "unset", "wire string")
		})
	})
}
