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

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("names an unstamped build rather than reporting nothing", func(t *testing.T) {
			t.Parallel()
			// A client logs this. An empty string reads as a missing
			// field; "dev" reads as a build nobody released.
			assert.Equal(t, version.String(), version.Dev,
				"a build without link-time stamps says so")
		})

		t.Run("never answers empty", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, version.String(), "a version a reader can act on is always reported")
		})
	})
}
