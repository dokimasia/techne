// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package version_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/version"
)

// TestDoc covers the claim the package comment makes about an unstamped
// build.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("an unstamped build", func(t *testing.T) {
		t.Parallel()

		t.Run("does not claim a release it is not", func(t *testing.T) {
			t.Parallel()
			assert.False(t, strings.HasPrefix(version.String(), "v"),
				"a version starting with v reads as a tag, which this build does not carry")
		})
	})
}
