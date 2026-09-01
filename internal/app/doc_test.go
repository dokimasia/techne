// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/app"
)

// TestDoc covers the claim the package comment makes: this is the one
// place that names a language, and the set is a value it chose.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the language set", func(t *testing.T) {
		t.Parallel()

		t.Run("is chosen here, not discovered", func(t *testing.T) {
			t.Parallel()
			// Registration is an explicit call rather than an init with
			// a blank import, so two servers built from the same binary
			// hold the same set and nothing else can add to it.
			first, err := app.Build(fstest.MapFS{})
			assert.NoError(t, err, "an empty workspace still registers every language")
			second, err := app.Build(fstest.MapFS{})
			assert.NoError(t, err, "an empty workspace still registers every language")
			assert.Equal(t, len(first.Languages), len(second.Languages),
				"the set does not depend on which packages happened to be imported")
		})

		t.Run("does not depend on what the workspace holds", func(t *testing.T) {
			t.Parallel()
			// A server serves a language whether or not the workspace
			// has a file in it, so capabilities answers the same either
			// way.
			empty, err := app.Build(fstest.MapFS{})
			assert.NoError(t, err, "an empty workspace still registers every language")
			assert.NotEmpty(t, empty.Languages, "a language is registered before any file is read")
		})
	})
}
