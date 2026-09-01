// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

func TestChange(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("does nothing to nothing", func(t *testing.T) {
			t.Parallel()
			// Applying an unset change is a programming error. It must
			// not read as any of the four real kinds.
			var unset edit.Change
			if unset.Kind != edit.ChangeUnset {
				t.Errorf("zero Change.Kind = %d, want ChangeUnset", unset.Kind)
			}
			if unset.Path != "" || len(unset.Edits) != 0 || unset.Content != nil {
				t.Errorf("zero Change carries work: %+v", unset)
			}
		})
	})

	t.Run("TextEdit", func(t *testing.T) {
		t.Parallel()

		t.Run("inserts where its span is empty", func(t *testing.T) {
			t.Parallel()
			// An insertion is a replacement of nothing, so the write
			// path needs no separate kind for it.
			at := source.Position{Offset: 40, Line: 3}
			insertion := edit.TextEdit{
				Span: source.Span{Path: "a/b.go", Start: at, End: at},
				New:  "// documented\n",
			}
			if insertion.Span.Start != insertion.Span.End {
				t.Error("an insertion must have an empty span")
			}
			if insertion.New == "" {
				t.Error("an insertion with no new text changes nothing")
			}
		})
	})
}
