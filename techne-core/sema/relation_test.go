// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestRelation(t *testing.T) {
	t.Parallel()

	t.Run("Inverse", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the same question from the other end", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Calls.Inverse(), sema.CalledBy,
				"who calls this is the same edge as what this calls, read the other way")
			assert.Equal(t, sema.CalledBy.Inverse(), sema.Calls,
				"who calls this is the same edge as what this calls, read the other way")
		})

		t.Run("is its own undo for every directed kind", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.RelationKinds() {
				assert.Equal(t, k.Inverse().Inverse(), k,
					"an engine stores one direction and a service turns it around, so the pairing round-trips")
			}
		})

		t.Run("pairs every directed kind with a different one", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.RelationKinds() {
				assert.NotEqual(t, k.Inverse(), k,
					"a kind that inverted to itself would leave a caller unable to ask the other way")
			}
		})

		t.Run("has no inverse for an unclassified edge", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.RelationUnknown.Inverse(), sema.RelationUnknown,
				"an edge nobody classified cannot be turned around")
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown", func(t *testing.T) {
			t.Parallel()
			var unset sema.RelationKind
			assert.Equal(t, unset, sema.RelationUnknown, "an unset edge claims no direction")
		})
	})
}
