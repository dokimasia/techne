// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/app"
)

func TestCommand(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the working directory for no argument", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse(nil)
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{}, "the command of no argument")
		})

		t.Run("returns the workspace of the first argument", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"/some/tree"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{Root: "/some/tree"}, "the command of one workspace")
		})

		t.Run("asks for the version for --version", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"--version"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{Version: true}, "the command of --version")
		})

		t.Run("returns an error for a flag other than --version", func(t *testing.T) {
			t.Parallel()
			_, err := app.Parse([]string{"--verbose"})
			assert.HasError(t, err, "the error of Parse")
			assert.Equal(t, err.Error(), `app: the command does not take the flag "--verbose"`, "the error of Parse")
		})

		t.Run("returns an error for a second workspace", func(t *testing.T) {
			t.Parallel()
			_, err := app.Parse([]string{"a", "b"})
			assert.HasError(t, err, "the error of Parse")
			assert.Equal(t, err.Error(), `app: the command takes one workspace, and "b" is a second`,
				"the error of Parse")
		})
	})
}
