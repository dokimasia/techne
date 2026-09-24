// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Position", func(t *testing.T) {
		t.Parallel()

		t.Run("counts the first byte as zero on every axis", func(t *testing.T) {
			t.Parallel()
			var first source.Position
			assert.Equal(t, []int{first.Offset, first.Line, first.Column}, []int{0, 0, 0}, "coordinates")
		})
	})
}
