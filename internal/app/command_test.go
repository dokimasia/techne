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

		trusting := []struct {
			name string
			give []string
			want app.Command
		}{
			{
				name: "returns the folder of --trust",
				give: []string{"--trust", "/projects", "/projects/a"},
				want: app.Command{Root: "/projects/a", Trusted: []string{"/projects"}},
			},
			{
				name: "returns the folder after --trust=",
				give: []string{"--trust=/projects"},
				want: app.Command{Trusted: []string{"/projects"}},
			},
			{
				name: "returns every folder of a repeated --trust in order",
				give: []string{"--trust", "/projects", "--trust=/work"},
				want: app.Command{Trusted: []string{"/projects", "/work"}},
			},
			{
				name: "returns a folder that starts with a hyphen after --trust=",
				give: []string{"--trust=-projects"},
				want: app.Command{Trusted: []string{"-projects"}},
			},
		}
		for _, tt := range trusting {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := app.Parse(tt.give)
				assert.NoError(t, err, "the error of Parse")
				assert.Equal(t, got, tt.want, "the command of the arguments")
			})
		}

		unfinished := []struct {
			name string
			give []string
		}{
			{name: "returns an error for --trust as the last argument", give: []string{"--trust"}},
			{name: "returns an error for --trust before a flag", give: []string{"--trust", "--structured"}},
			{name: "returns an error for --trust= without a folder", give: []string{"--trust="}},
		}
		for _, tt := range unfinished {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := app.Parse(tt.give)
				assert.HasError(t, err, "the error of Parse")
				assert.Equal(t, err.Error(), "app: --trust takes a folder", "the error of Parse")
			})
		}
	})
}
