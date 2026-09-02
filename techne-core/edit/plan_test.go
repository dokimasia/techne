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

		t.Run("names both ends of a move", func(t *testing.T) {
			t.Parallel()
			// A move touches the file it leaves and the file it becomes.
			// Both need a lock and both must be free to write, so a plan
			// naming only one would hold half of what it changes.
			got := edit.Plan{Changes: []edit.Change{
				{Kind: edit.ChangeMove, Path: "b.go", To: "a.go"},
			}}.Paths()
			assert.Equal(t, got, []source.Path{"a.go", "b.go"},
				"a move changes two paths, and the plan holds both")
		})

		t.Run("sorts, so two callers take locks in one order", func(t *testing.T) {
			t.Parallel()
			got := edit.Plan{Changes: []edit.Change{
				{Kind: edit.ChangeEdit, Path: "z.go"},
				{Kind: edit.ChangeEdit, Path: "a.go"},
				{Kind: edit.ChangeEdit, Path: "m.go"},
			}}.Paths()
			assert.Equal(t, got, []source.Path{"a.go", "m.go", "z.go"},
				"two changes touching the same files in different orders would deadlock")
		})

		t.Run("names a path once however often it is changed", func(t *testing.T) {
			t.Parallel()
			got := edit.Plan{Changes: []edit.Change{
				{Kind: edit.ChangeEdit, Path: "a.go"},
				{Kind: edit.ChangeEdit, Path: "a.go"},
			}}.Paths()
			assert.Length(t, got, 1, "a path taken twice would be locked twice and deadlock on itself")
		})
	})

	t.Run("Reads", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves out a file the plan creates", func(t *testing.T) {
			t.Parallel()
			// A file that does not exist has no content to depend on.
			// Pinning one would refuse every plan that makes a file.
			got := edit.Plan{Changes: []edit.Change{
				{Kind: edit.ChangeCreate, Path: "new.go", Content: []byte("package a\n")},
				{Kind: edit.ChangeEdit, Path: "old.go"},
			}}.Reads()
			assert.Equal(t, got, []source.Path{"old.go"},
				"only a file the plan read can be pinned to what was read")
		})

		t.Run("holds the file a move leaves", func(t *testing.T) {
			t.Parallel()
			got := edit.Plan{Changes: []edit.Change{
				{Kind: edit.ChangeMove, Path: "b.go", To: "a.go"},
			}}.Reads()
			assert.Equal(t, got, []source.Path{"b.go"},
				"a move carries the content it read, so that content is what it depends on")
		})
	})
}
