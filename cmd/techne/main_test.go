// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/version"
)

// TestMain covers what this command decides. Serving is exercised
// end to end against a real client in the presenter's own tests; what is
// left here is the shape of the entry point.
func TestMain(t *testing.T) {
	t.Parallel()

	t.Run("version", func(t *testing.T) {
		t.Parallel()

		t.Run("is reported to the client rather than left empty", func(t *testing.T) {
			t.Parallel()
			// A client logs what it connected to. An empty string reads
			// as a missing field.
			assert.NotEmpty(t, version.String(), "a server names the build a client connected to")
		})
	})

	t.Run("workspace root", func(t *testing.T) {
		t.Parallel()

		t.Run("defaults to the working directory when none is given", func(t *testing.T) {
			t.Parallel()
			// The argument is optional, so an agent launching the server
			// with no arguments still gets its own tree.
			assert.Empty(t, rootFrom(nil), "no argument means the working directory")
			assert.Equal(t, rootFrom([]string{"techne", "/some/tree"}), "/some/tree",
				"the first argument is the workspace root")
		})
	})
}
