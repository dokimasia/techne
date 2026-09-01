// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"testing"

	"go.dokimi.dev/techne/core/source"
)

// TestDoc covers the contracts the package comment states and no single
// declaration owns.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("coordinates", func(t *testing.T) {
		t.Parallel()

		t.Run("are zero-based, so the first byte is the zero Position", func(t *testing.T) {
			t.Parallel()
			var first source.Position
			if first.Offset != 0 || first.Line != 0 || first.Column != 0 {
				t.Errorf("zero Position = %+v, want all three zero", first)
			}
		})
	})
}
