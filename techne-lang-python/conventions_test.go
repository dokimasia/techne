// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/python"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads what pytest and unittest both discover", func(t *testing.T) {
			t.Parallel()
			assert.True(t, python.IsTest("tests/test_store.py"), "a test_ prefix is discovered")
			assert.True(t, python.IsTest("pkg/store_test.py"), "a _test suffix is discovered")
			assert.False(t, python.IsTest("pkg/store.py"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the underscore convention", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Visibility("Store"), sema.Exported,
				"a name without a leading underscore is part of the module's surface")
			assert.Equal(t, python.Visibility("_helper"), sema.Unexported,
				"a leading underscore marks a name as internal")
		})

		t.Run("reports unknown for a name that is not one", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, python.Visibility(""), sema.VisibilityUnknown,
				"an empty name carries no convention to read")
		})
	})
}
