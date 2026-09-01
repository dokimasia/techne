// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
)

func TestRelation(t *testing.T) {
	t.Parallel()

	t.Run("Inverse", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the same question from the other end", func(t *testing.T) {
			t.Parallel()
			if got := sema.Calls.Inverse(); got != sema.CalledBy {
				t.Errorf("Calls.Inverse() = %d, want CalledBy", got)
			}
			if got := sema.CalledBy.Inverse(); got != sema.Calls {
				t.Errorf("CalledBy.Inverse() = %d, want Calls", got)
			}
		})

		t.Run("is its own undo for every directed kind", func(t *testing.T) {
			t.Parallel()
			// An engine offers one direction and the service asks for
			// the other. A kind added without its pair breaks that
			// silently, so the property is asserted over the whole set
			// rather than a sample.
			for _, k := range sema.RelationKinds() {
				if k.Inverse().Inverse() != k {
					t.Errorf("kind %d does not survive a round trip: got %d", k, k.Inverse().Inverse())
				}
			}
		})

		t.Run("pairs every directed kind with a different one", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.RelationKinds() {
				if k.Inverse() == k {
					t.Errorf("kind %d is its own inverse, so a caller cannot ask the other way", k)
				}
			}
		})

		t.Run("has no inverse for an unclassified edge", func(t *testing.T) {
			t.Parallel()
			if got := sema.RelationUnknown.Inverse(); got != sema.RelationUnknown {
				t.Errorf("RelationUnknown.Inverse() = %d, want RelationUnknown", got)
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown", func(t *testing.T) {
			t.Parallel()
			var unset sema.RelationKind
			if unset != sema.RelationUnknown {
				t.Errorf("zero RelationKind = %d, want RelationUnknown", unset)
			}
		})
	})
}
