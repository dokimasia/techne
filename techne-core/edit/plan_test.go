// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Paths", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []edit.Change
			want []source.Path
		}{
			{
				name: "returns the source and destination of a move",
				give: []edit.Change{move("b.go", "a.go")},
				want: []source.Path{"a.go", "b.go"},
			},
			{
				name: "returns the paths sorted",
				give: []edit.Change{
					{Kind: edit.ChangeEdit, Path: "z.go"},
					{Kind: edit.ChangeEdit, Path: "a.go"},
					{Kind: edit.ChangeEdit, Path: "m.go"},
				},
				want: []source.Path{"a.go", "m.go", "z.go"},
			},
			{
				name: "returns a repeated path once",
				give: []edit.Change{
					{Kind: edit.ChangeEdit, Path: "a.go"},
					{Kind: edit.ChangeEdit, Path: "a.go"},
				},
				want: []source.Path{"a.go"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, edit.Plan{Changes: tt.give}.Paths(), tt.want, "paths")
			})
		}
	})

	t.Run("Reads", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []edit.Change
			want []source.Path
		}{
			{
				name: "omits a path the plan creates",
				give: []edit.Change{
					{Kind: edit.ChangeCreate, Path: "new.go", Content: []byte("package a\n")},
					{Kind: edit.ChangeEdit, Path: "old.go"},
				},
				want: []source.Path{"old.go"},
			},
			{
				name: "returns the source of a move",
				give: []edit.Change{move("b.go", "a.go")},
				want: []source.Path{"b.go"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, edit.Plan{Changes: tt.give}.Reads(), tt.want, "reads")
			})
		}
	})
}
