// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestNode(t *testing.T) {
	t.Parallel()

	t.Run("NodeMajor", func(t *testing.T) {
		t.Parallel()

		manifest := func(json string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(json)} }
		tests := []struct {
			name  string
			files fstest.MapFS
			want  int
			found bool
		}{
			{
				name: "returns the version of the installed package",
				files: fstest.MapFS{
					"node_modules/typescript/package.json": manifest(`{"version": "6.0.3"}`),
					"package.json":                         manifest(`{"devDependencies": {"typescript": "^7.0.2"}}`),
				},
				want: 6, found: true,
			},
			{
				name:  "returns the range of the development dependencies",
				files: fstest.MapFS{"package.json": manifest(`{"devDependencies": {"typescript": "^7.0.2"}}`)},
				want:  7, found: true,
			},
			{
				name:  "returns the range of the dependencies",
				files: fstest.MapFS{"package.json": manifest(`{"dependencies": {"typescript": "~5.9.3"}}`)},
				want:  5, found: true,
			},
			{
				name: "reads the range of an alias",
				files: fstest.MapFS{
					"package.json": manifest(`{"dependencies": {"typescript": "npm:typescript@7.1.0"}}`),
				},
				want: 7, found: true,
			},
			{
				name: "returns the range of the catalog of the workspaces",
				files: fstest.MapFS{"package.json": manifest(`{
					"workspaces": {"catalog": {"typescript": "^7.0.2"}, "packages": ["ui/*"]},
					"devDependencies": {"typescript": "catalog:"}
				}`)},
				want: 7, found: true,
			},
			{
				name:  "returns the range of the catalog of the root",
				files: fstest.MapFS{"package.json": manifest(`{"catalog": {"typescript": "7.1.0"}}`)},
				want:  7, found: true,
			},
			{
				name:  "reads workspaces written as a list of patterns",
				files: fstest.MapFS{"package.json": manifest(`{"workspaces": ["packages/*"]}`)},
			},
			{
				name:  "returns false for a range without a number",
				files: fstest.MapFS{"package.json": manifest(`{"devDependencies": {"typescript": "latest"}}`)},
			},
			{
				name:  "returns false for a workspace without the package",
				files: fstest.MapFS{"package.json": manifest(`{"dependencies": {"react": "^19.0.0"}}`)},
			},
			{
				name:  "returns false for a workspace without a package.json",
				files: fstest.MapFS{"main.go": &fstest.MapFile{Data: []byte("package main\n")}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				major, found := lang.NodeMajor(tt.files, "typescript")
				assert.Equal(t, found, tt.found, "whether the workspace names typescript")
				assert.Equal(t, major, tt.want, "the major version of typescript")
			})
		}

		t.Run("returns false for a workspace without a tree", func(t *testing.T) {
			t.Parallel()
			_, found := lang.NodeMajor(nil, "typescript")
			assert.False(t, found, "whether a nil tree names typescript")
		})
	})
}
