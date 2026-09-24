// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

func TestProject(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"go.mod":                  {Data: []byte("module root\n")},
		"main.go":                 {Data: []byte("package main")},
		"lang/go.mod":             {Data: []byte("module lang\n")},
		"lang/read.go":            {Data: []byte("package lang")},
		"lang/lsp/engine.go":      {Data: []byte("package lsp")},
		"lang-go/go.mod":          {Data: []byte("module langgo\n")},
		"lang-go/checker/load.go": {Data: []byte("package checker")},
		"notes/note.go":           {Data: []byte("package notes")},
		"app/App.csproj":          {Data: []byte("<Project/>")},
		"app/Program.cs":          {Data: []byte("class P {}")},
		"odd/go.mod/marker":       {Data: []byte("")},
		"odd/main.go":             {Data: []byte("package main")},
	}

	t.Run("ProjectOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      source.Path
			manifests []string
			want      source.Path
		}{
			{
				name:      "returns the directory of the nearest manifest",
				give:      "lang/lsp/engine.go",
				manifests: []string{"go.mod"},
				want:      "lang",
			},
			{
				name:      "returns a sibling module that shares a prefix",
				give:      "lang-go/checker/load.go",
				manifests: []string{"go.mod"},
				want:      "lang-go",
			},
			{
				name:      "returns the root for a file under the root manifest",
				give:      "notes/note.go",
				manifests: []string{"go.mod"},
				want:      engine.Root,
			},
			{
				name:      "matches a manifest pattern",
				give:      "app/Program.cs",
				manifests: []string{"*.sln", "*.csproj"},
				want:      "app",
			},
			{
				name:      "skips a directory named like a manifest",
				give:      "odd/main.go",
				manifests: []string{"go.mod"},
				want:      engine.Root,
			},
			{name: "returns the root without manifests", give: "lang/read.go", want: engine.Root},
			{
				name:      "returns the root for an absolute path",
				give:      "/usr/lib/go/src/fmt/print.go",
				manifests: []string{"go.mod"},
				want:      engine.Root,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.ProjectOf(fsys, tt.give, tt.manifests), tt.want, "ProjectOf")
			})
		}
	})

	t.Run("Within", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    source.Path
			project source.Path
			want    bool
		}{
			{
				name:    "returns true for a path under the project",
				give:    "lang/lsp/engine.go",
				project: "lang",
				want:    true,
			},
			{name: "returns true for the project itself", give: "lang", project: "lang", want: true},
			{
				name:    "returns false for a sibling that shares a prefix",
				give:    "lang-go/checker/load.go",
				project: "lang",
			},
			{
				name:    "returns true for every path within the root",
				give:    "any/where.go",
				project: engine.Root,
				want:    true,
			},
			{
				name:    "returns false for a path outside the workspace",
				give:    "/home/dev/.cache/typescript/node_modules/@types/node/fs.d.ts",
				project: engine.Root,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Within(tt.give, tt.project), tt.want, "Within")
			})
		}
	})
}
