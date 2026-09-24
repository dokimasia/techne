// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("RelationKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("lists the inverse of every listed kind", func(t *testing.T) {
			t.Parallel()
			kinds := sema.RelationKinds()
			for _, k := range kinds {
				assert.True(t, slices.Contains(kinds, k.Inverse()), k.String())
			}
		})
	})
}
