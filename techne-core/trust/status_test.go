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

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is not OK", func(t *testing.T) {
			t.Parallel()
			var unset trust.Status
			assert.NotEqual(t, unset, trust.OK,
				"an answer whose status nobody assigned must not read as a success")
		})

		t.Run("reports that nothing answered", func(t *testing.T) {
			t.Parallel()
			var unset trust.Status
			assert.False(t, unset.Answered(),
				"an unset status carries no payload, so its empty item list proves nothing")
		})
	})

	t.Run("Answered", func(t *testing.T) {
		t.Parallel()

		t.Run("is true where a payload was produced", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.OK, trust.Degraded, trust.Partial} {
				assert.True(t, s.Answered(),
					"an engine ran and returned items worth reading, however qualified")
			}
		})

		t.Run("is false where nothing ran", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.Unset, trust.Unsupported, trust.Refused} {
				assert.False(t, s.Answered(),
					"nothing ran, so an empty item list is not evidence of absence")
			}
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of the status", func(t *testing.T) {
			t.Parallel()
			for st, want := range map[trust.Status]string{
				trust.Unset: "unset", trust.OK: "ok", trust.Degraded: "degraded",
				trust.Partial: "partial", trust.Unsupported: "unsupported", trust.Refused: "refused",
			} {
				assert.Equal(t, st.String(), want, "an answer carries this string to a caller")
			}
			assert.Equal(t, trust.Status(200).String(), "unset",
				"a status outside the set reads as unset rather than as a success")
		})
	})

	t.Run("distinctness", func(t *testing.T) {
		t.Parallel()

		t.Run("unsupported differs from refused", func(t *testing.T) {
			t.Parallel()
			assert.NotEqual(t, trust.Unsupported, trust.Refused,
				"one is a capability gap to route around, the other a request to correct")
		})
	})
}
