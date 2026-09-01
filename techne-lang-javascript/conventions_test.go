// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/javascript"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the naming the language's own tooling discovers by", func(t *testing.T) {
			t.Parallel()
			assert.True(t, javascript.IsTest("src/store.test.js"), "the runners discover by this naming")
			assert.True(t, javascript.IsTest("src/__tests__/store.js"), "the runners discover by this directory")
			assert.False(t, javascript.IsTest("src/store.js"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown, because the name does not carry the rule", func(t *testing.T) {
			t.Parallel()
			// Reporting a guess would make a caller filtering to public
			// API drop declarations it should have kept.
			assert.Equal(t, javascript.Visibility("Store"), sema.VisibilityUnknown,
				"the export keyword decides this, and a name carries nothing of it")
			assert.Equal(t, javascript.Visibility("store"), sema.VisibilityUnknown,
				"the export keyword decides this, and a name carries nothing of it")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("drops the extension, leaving what an import names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, javascript.Namespace("src/store.js"), "src/store",
				"two symbols in one file share a unit, and the extension is not part of its name")
		})
	})
}
