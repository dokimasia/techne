// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
)

func TestSpan(t *testing.T) {
	t.Parallel()

	t.Run("Span", func(t *testing.T) {
		t.Parallel()

		t.Run("names no file when zero", func(t *testing.T) {
			t.Parallel()
			var zero source.Span
			assert.Equal(t, zero.Path, source.Path(""), "path")
		})

		t.Run("encodes its fields under lowercase JSON names", func(t *testing.T) {
			t.Parallel()
			span := source.Span{
				Path:  "a/b.go",
				Start: source.Position{Offset: 12, Line: 1, Column: 4},
				End:   source.Position{Offset: 15, Line: 1, Column: 7},
			}
			encoded, err := json.Marshal(span)
			assert.NoError(t, err, "marshal")
			assert.Equal(t, string(encoded),
				`{"path":"a/b.go","start":{"offset":12,"line":1,"column":4},"end":{"offset":15,"line":1,"column":7}}`,
				"encoding")
		})
	})
}
