// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package version_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("returns dev for an empty version", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("", "abc123", "2026-01-01"), "dev", "the version string")
		})

		t.Run("returns the version for an empty commit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("1.2.3", "", "2026-01-01"), "1.2.3", "the version string")
		})

		t.Run("returns the version with the commit for an empty date", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("1.2.3", "abc123", ""), "1.2.3 (abc123)", "the version string")
		})

		t.Run("returns the date of the build beside the commit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("1.2.3", "abc123", "2026-01-01"), "1.2.3 (abc123, built 2026-01-01)",
				"the version string")
		})
	})

	t.Run("Full", func(t *testing.T) {
		t.Parallel()

		t.Run("returns dev for a build without the flags", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Full(), "dev", "the version of the test binary")
		})
	})
}
