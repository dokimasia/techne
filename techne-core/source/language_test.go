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

	t.Run("the set", func(t *testing.T) {
		t.Parallel()

		t.Run("is open, so core declares no language", func(t *testing.T) {
			t.Parallel()
			// A language module owns its own value. Were core to declare
			// one, deleting that module would leave the language named
			// here and the deletion would not be complete.
			registered := source.Language("a-language-core-never-heard-of")
			if registered == "" {
				t.Error("any non-empty string must be a usable Language")
			}
		})

		t.Run("keys a registry, so a duplicate claim is caught", func(t *testing.T) {
			t.Parallel()
			// Registration rejects a second module claiming a language
			// already taken. That check is a lookup, so the type has to
			// work as a map key and compare by wire form.
			taken := map[source.Language]string{}
			taken[source.Language("go")] = "the first module"

			if _, clash := taken[source.Language("go")]; !clash {
				t.Error("a second claim on one wire form must be found")
			}
			if _, clash := taken[source.Language("golang")]; clash {
				t.Error("a different wire form must not read as taken")
			}
		})
	})
}
