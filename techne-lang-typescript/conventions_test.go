// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/typescript"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads what Jest and Vitest both discover", func(t *testing.T) {
			t.Parallel()
			assert.True(t, typescript.IsTest("src/store.test.ts"), "the test infix")
			assert.True(t, typescript.IsTest("src/store.spec.ts"), "the spec infix")
			assert.True(t, typescript.IsTest("src/__tests__/store.ts"), "the tests directory")
			assert.False(t, typescript.IsTest("src/store.ts"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is always unknown, because the export keyword carries it", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "helper", ""} {
				assert.Equal(t, typescript.Visibility(name), sema.VisibilityUnknown,
					"a tags query captures the name, and TypeScript spells visibility with export")
			}
		})
	})
}
