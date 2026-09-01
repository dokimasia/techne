// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	golang "go.dokimi.dev/techne/lang/go"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the suffix the toolchain itself uses", func(t *testing.T) {
			t.Parallel()
			assert.True(t, golang.IsTest("pkg/store_test.go"), "go test discovers by this suffix")
			assert.False(t, golang.IsTest("pkg/store.go"), "shipped code is not a test")
			assert.False(t, golang.IsTest("pkg/testing.go"), "a name containing test is not the suffix")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is never unknown, because the name carries the whole rule", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Visibility("Store"), sema.Exported,
				"an upper-case initial makes a Go declaration visible outside its package")
			assert.Equal(t, golang.Visibility("helper"), sema.Unexported,
				"a lower-case initial keeps a Go declaration inside its package")
		})

		t.Run("reports unknown for a name that is not one", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Visibility(""), sema.VisibilityUnknown,
				"an empty name carries no rule to read")
		})
	})

	t.Run("Unit", func(t *testing.T) {
		t.Parallel()

		t.Run("is the directory, because a Go package is one", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Unit("core/trust/status.go"), "core/trust",
				"two files in one directory belong to one unit, so their symbols share it")
			assert.Equal(t, golang.Unit("core/trust/evidence.go"), "core/trust",
				"two files in one directory belong to one unit, so their symbols share it")
		})
	})
}
