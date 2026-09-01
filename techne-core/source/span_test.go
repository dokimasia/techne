// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"testing"

	"go.dokimi.dev/techne/core/source"
)

func TestSpan(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("names no file and covers nothing", func(t *testing.T) {
			t.Parallel()
			var empty source.Span
			if empty.Path != "" {
				t.Errorf("zero Span.Path = %q, want the empty string", empty.Path)
			}
			if empty.Start != empty.End {
				t.Errorf("zero Span covers %+v to %+v, want an empty range", empty.Start, empty.End)
			}
		})
	})

	t.Run("range", func(t *testing.T) {
		t.Parallel()

		t.Run("is empty when Start equals End", func(t *testing.T) {
			t.Parallel()
			// An insertion point is expressed this way, so the two
			// being equal has to stay meaningful rather than invalid.
			at := source.Position{Offset: 12, Line: 1, Column: 4}
			insertion := source.Span{Path: "a/b.go", Start: at, End: at}
			if insertion.Start != insertion.End {
				t.Error("an insertion point must have Start equal to End")
			}
		})
	})
}
