// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"errors"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// moving returns the change that moves a.fx to to.
func moving(to source.Path) edit.Change {
	return edit.Change{Kind: edit.ChangeMove, Path: "a.fx", To: to}
}

// twice returns the changes that insert the comment Doc. at the start of a.fx and of b.fx.
func twice() []edit.Change {
	return append(comment("a.fx", "Doc."), comment("b.fx", "Doc.")...)
}

// relocated returns the changes that insert the comment Doc. at the start of a.fx and move
// a.fx to b.fx.
func relocated() []edit.Change {
	return append(comment("a.fx", "Doc."), moving("b.fx"))
}

// made returns the change that creates b.fx.
func made() []edit.Change {
	return []edit.Change{{Kind: edit.ChangeCreate, Path: "b.fx", Content: []byte("made\n")}}
}

func TestApply(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("puts back the files already written when a write fails", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: twice()}, clean())
			files.put("b.fx", original)
			files.refuse = "b.fx"
			_, err := s.Apply(t.Context(), asking(false))
			assert.HasError(t, err, "the error of Apply")
			assert.Contains(t, err.Error(), "the files already written were put back", "the error of Apply")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
			assert.Equal(t, files.at("b.fx"), original, "the content of b.fx")
		})

		t.Run("writes the edit of a moved file at its destination", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: relocated()}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Changed, []source.Path{"a.fx", "b.fx"}, "the files written")
			assert.Equal(t, files.at("b.fx"), "// Doc.\n"+original, "the content of b.fx")
			assert.False(t, files.has("a.fx"), "the file at a.fx")
		})

		t.Run("moves a file without writing its content", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: []edit.Change{moving("b.fx")}}, clean())
			_, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, files.at("b.fx"), original, "the content of b.fx")
			assert.Equal(t, files.writes("b.fx"), 0, "the writes of b.fx")
			assert.Equal(t, files.moves, 1, "the moves of the workspace")
		})

		t.Run("puts a moved file back when the write of its destination fails", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: relocated()}, clean())
			files.refuse = "b.fx"
			_, err := s.Apply(t.Context(), asking(false))
			assert.HasError(t, err, "the error of Apply")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
			assert.False(t, files.has("b.fx"), "the file at b.fx")
		})

		t.Run("moves a file once for a move that the plan repeats", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: []edit.Change{moving("b.fx"), moving("b.fx")}}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, files.at("b.fx"), original, "the content of b.fx")
			assert.Equal(t, files.moves, 1, "the moves of the workspace")
		})

		t.Run("refuses a plan that moves a file to two destinations", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{changes: []edit.Change{moving("b.fx"), moving("c.fx")}}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Contains(t, got.Reason, "moved to both", "the reason of the refusal")
		})

		t.Run("writes an empty file for a create without content", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: []edit.Change{{Kind: edit.ChangeCreate, Path: "b.fx"}}}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Changed, []source.Path{"b.fx"}, "the files written")
			assert.True(t, files.has("b.fx"), "the file at b.fx")
			assert.Empty(t, files.at("b.fx"), "the content of b.fx")
		})

		t.Run("refuses a create of a path with a file", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: made()}, clean())
			files.put("b.fx", "kept\n")
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Contains(t, got.Reason, "b.fx", "the reason of the refusal")
			assert.Equal(t, files.at("b.fx"), "kept\n", "the content of b.fx")
		})

		t.Run("refuses a dry run of a create of a path with a file", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: made()}, clean())
			files.put("b.fx", "kept\n")
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Empty(t, got.Handle, "the handle of the outcome")
		})

		t.Run("refuses a move onto a path with a file", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: []edit.Change{moving("b.fx")}}, clean())
			files.put("b.fx", "kept\n")
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at("b.fx"), "kept\n", "the content of b.fx")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
		})

		t.Run("refuses a change to a file that changed after the plan read it", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			files.meanwhile = func(w *workspace) { w.put("a.fx", "somebody else\n") }
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, got.Reason, "a.fx changed after the plan read it, and nothing had been written",
				"the reason of the refusal")
			assert.Equal(t, files.at("a.fx"), "somebody else\n", "the content of a.fx")
		})

		t.Run("puts back the files already written when a later file changed", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: twice()}, clean())
			files.put("b.fx", original)
			files.meanwhile = func(w *workspace) { w.put("b.fx", "somebody else\n") }
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Reason,
				"b.fx changed after the plan read it, and the files already written were put back",
				"the reason of the refusal")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
			assert.Equal(t, files.at("b.fx"), "somebody else\n", "the content of b.fx")
		})

		t.Run("refuses a create of a path that a file took after the plan", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: made()}, clean())
			files.meanwhile = func(w *workspace) { w.put("b.fx", "somebody else\n") }
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at("b.fx"), "somebody else\n", "the content of b.fx")
		})

		t.Run("writes every file under the lock of the workspace", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: append(relocated(), comment("c.fx", "Doc.")...)}, clean())
			files.put("c.fx", original)
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, files.outside, 0, "the changes made without the lock")
		})

		t.Run("returns an error when it cannot take the lock of the workspace", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			files.jammed = errors.New("the lock file is on a read-only disk")
			_, err := s.Apply(t.Context(), asking(false))
			assert.ErrorIs(t, err, files.jammed, "the error of Apply")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
		})

		t.Run("names no file for a plan whose result equals the content", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{changes: []edit.Change{replacing("a.fx", 0, 4, "one\n")}}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Empty(t, got.Changed, "the files written")
			assert.Equal(t, files.writes("a.fx"), 0, "the writes of a.fx")
		})

		t.Run("writes the changes of concurrent callers of one file in turn", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					_, err := s.Apply(t.Context(), asking(false))
					assert.NoError(t, err, "Apply")
				})
			}
			wg.Wait()
			assert.Equal(t, files.outside, 0, "the changes made without the lock")
		})
	})
}
