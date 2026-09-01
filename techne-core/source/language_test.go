// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"testing"

	"go.dokimi.dev/techne/core/source"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("names nothing", func(t *testing.T) {
			t.Parallel()
			var unset source.Language
			if unset != "" {
				t.Errorf("zero Language = %q, want the empty string", unset)
			}
		})
	})

	t.Run("wire form", func(t *testing.T) {
		t.Parallel()

		// These strings reach a caller and, through a sema identity, an
		// index that outlives the process. Changing one invalidates
		// stored data, so the test pins them rather than deriving them.
		t.Run("is the lowercase language name", func(t *testing.T) {
			t.Parallel()
			for got, want := range map[source.Language]string{
				source.Go:         "go",
				source.Python:     "python",
				source.Java:       "java",
				source.Rust:       "rust",
				source.TypeScript: "typescript",
			} {
				if string(got) != want {
					t.Errorf("language = %q, want %q", string(got), want)
				}
			}
		})

		t.Run("differs between languages", func(t *testing.T) {
			t.Parallel()
			seen := map[source.Language]bool{}
			for _, l := range []source.Language{
				source.Go, source.Python, source.Java, source.Rust, source.TypeScript,
			} {
				if seen[l] {
					t.Errorf("language %q is declared twice", l)
				}
				seen[l] = true
			}
		})
	})
}
