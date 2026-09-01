// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/rust"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads Cargo's layout", func(t *testing.T) {
			t.Parallel()
			assert.True(t, rust.IsTest("tests/integration.rs"), "Cargo's integration test directory")
			assert.True(t, rust.IsTest("crate/tests/api.rs"), "a nested test directory")
			assert.False(t, rust.IsTest("src/lib.rs"), "shipped code is not a test")
		})

		t.Run("cannot see a unit test inside a source file", func(t *testing.T) {
			t.Parallel()
			// A #[cfg(test)] module lives in the file it tests, and a
			// path carries nothing of it.
			assert.False(t, rust.IsTest("src/store.rs"),
				"a path cannot report a test module declared inside a source file")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is always unknown, because pub carries it", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "helper", ""} {
				assert.Equal(t, rust.Visibility(name), sema.VisibilityUnknown,
					"a tags query captures the name, and Rust spells visibility with a pub modifier")
			}
		})
	})
}
