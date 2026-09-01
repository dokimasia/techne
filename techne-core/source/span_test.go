// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
)

func TestSpan(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("names no file and covers nothing", func(t *testing.T) {
			t.Parallel()
			var empty source.Span
			assert.Empty(t, string(empty.Path), "an unset span names no file")
			assert.Equal(t, empty.Start, empty.End, "an unset span covers no bytes")
		})
	})

	t.Run("range", func(t *testing.T) {
		t.Parallel()

		t.Run("is empty when Start equals End", func(t *testing.T) {
			t.Parallel()
			at := source.Position{Offset: 12, Line: 1, Column: 4}
			insertion := source.Span{Path: "a/b.go", Start: at, End: at}
			assert.Equal(t, insertion.Start, insertion.End,
				"an insertion point is a replacement of nothing, so the two ends meet")
		})
	})
}
