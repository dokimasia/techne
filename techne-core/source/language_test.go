// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("names nothing", func(t *testing.T) {
			t.Parallel()
			var unset source.Language
			assert.Empty(t, string(unset), "an unset language names nothing")
		})
	})

	t.Run("the set", func(t *testing.T) {
		t.Parallel()

		t.Run("is open, so core declares no language", func(t *testing.T) {
			t.Parallel()
			registered := source.Language("a-language-core-never-heard-of")
			assert.NotEmpty(t, string(registered),
				"a language module owns its own value, so any non-empty string is usable")
		})

		t.Run("keys a registry, so a duplicate claim is caught", func(t *testing.T) {
			t.Parallel()
			taken := map[source.Language]string{source.Language("go"): "the first module"}

			_, clash := taken[source.Language("go")]
			assert.True(t, clash, "a second module claiming one wire form is found by lookup")

			_, other := taken[source.Language("golang")]
			assert.False(t, other, "a different wire form is a different language")
		})
	})
}
