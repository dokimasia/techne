// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
)

func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("Suite", func(t *testing.T) {
		t.Parallel()

		t.Run("has no files and no declarations when zero", func(t *testing.T) {
			t.Parallel()
			var zero conformance.Suite
			assert.Empty(t, zero.Files, "Files")
			assert.Empty(t, zero.Declares, "Declares")
		})
	})

	t.Run("Declared", func(t *testing.T) {
		t.Parallel()

		t.Run("expects VisibilityUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero conformance.Declared
			assert.Equal(t, zero.Visibility, sema.VisibilityUnknown, "Visibility")
		})
	})
}
