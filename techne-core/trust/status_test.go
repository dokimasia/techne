// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/techne/core/trust"
)

func TestStatus(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is not OK", func(t *testing.T) {
			t.Parallel()
			// A service that forgets to set a status must not produce an
			// answer that reads as successful.
			var unset trust.Status
			if unset == trust.OK {
				t.Error("the zero Status must not be OK")
			}
		})

		t.Run("reports that nothing answered", func(t *testing.T) {
			t.Parallel()
			var unset trust.Status
			if unset.Answered() {
				t.Error("the zero Status must not report a payload")
			}
		})
	})

	t.Run("Answered", func(t *testing.T) {
		t.Parallel()

		t.Run("is true where a payload was produced", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.OK, trust.Degraded, trust.Partial} {
				if !s.Answered() {
					t.Errorf("status %d must report a payload", s)
				}
			}
		})

		t.Run("is false where nothing ran", func(t *testing.T) {
			t.Parallel()
			for _, s := range []trust.Status{trust.Unset, trust.Unsupported, trust.Refused} {
				if s.Answered() {
					t.Errorf("status %d must not report a payload", s)
				}
			}
		})
	})

	t.Run("distinctness", func(t *testing.T) {
		t.Parallel()

		t.Run("unsupported differs from refused", func(t *testing.T) {
			t.Parallel()
			// They license opposite next actions: route around a
			// capability gap, or correct the request.
			if trust.Unsupported == trust.Refused {
				t.Error("unsupported and refused must be distinct")
			}
		})
	})
}
