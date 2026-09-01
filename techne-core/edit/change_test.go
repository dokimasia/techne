// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

func TestChange(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("does nothing to nothing", func(t *testing.T) {
			t.Parallel()
			var unset edit.Change
			assert.Equal(t, unset.Kind, edit.ChangeUnset,
				"an unset change must not read as any of the four that touch a file")
			assert.Empty(t, string(unset.Path), "an unset change names no file")
			assert.Empty(t, unset.Edits, "an unset change rewrites nothing")
			assert.Nil(t, unset.Content, "an unset change writes no content")
		})
	})

	t.Run("TextEdit", func(t *testing.T) {
		t.Parallel()

		t.Run("inserts where its span is empty", func(t *testing.T) {
			t.Parallel()
			at := source.Position{Offset: 40, Line: 3}
			insertion := edit.TextEdit{
				Span: source.Span{Path: "a/b.go", Start: at, End: at},
				New:  "// documented\n",
			}
			assert.Equal(t, insertion.Span.Start, insertion.Span.End,
				"an insertion is a replacement of nothing, so the write path needs no separate kind")
			assert.NotEmpty(t, insertion.New, "an insertion with no new text changes nothing")
		})
	})
}
