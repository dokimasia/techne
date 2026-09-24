// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("KindOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns KindStruct for a class and a struct", func(t *testing.T) {
			t.Parallel()
			for _, c := range []treesitter.Capture{treesitter.DefinitionClass, treesitter.DefinitionStruct} {
				kind, _ := treesitter.KindOf(c)
				assert.Equal(t, kind, sema.KindStruct, string(c))
			}
		})
	})

	t.Run("Definitions", func(t *testing.T) {
		t.Parallel()

		t.Run("names no language", func(t *testing.T) {
			t.Parallel()
			languages := []string{
				"c", "csharp", "go", "java", "javascript", "python", "ruby", "rust", "scala", "typescript",
			}
			for _, c := range treesitter.Definitions() {
				kind := strings.TrimPrefix(string(c), treesitter.DefinitionPrefix)
				for _, language := range languages {
					assert.False(t, strings.EqualFold(kind, language), string(c))
				}
			}
		})
	})
}
