// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/app"
	"go.dokimi.dev/techne/presenter"
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

		t.Run("asks for text results without a flag", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"/some/tree"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got.Output, presenter.Text, "the output of a command without --structured")
		})

		t.Run("asks for structured results for --structured", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"--structured", "/some/tree"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{Root: "/some/tree", Output: presenter.Structured},
				"the command of --structured")
		})

		t.Run("asks for the usage for --help", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"--help"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{Help: true}, "the command of --help")
		})

		t.Run("asks for the usage for -h", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"-h"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{Help: true}, "the command of -h")
		})

		t.Run("reads no argument after a help flag", func(t *testing.T) {
			t.Parallel()
			got, err := app.Parse([]string{"--version", "--help", "--verbose", "a", "b"})
			assert.NoError(t, err, "the error of Parse")
			assert.Equal(t, got, app.Command{Help: true}, "the command of --help before other arguments")
		})

		t.Run("returns an error for an undeclared flag", func(t *testing.T) {
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
