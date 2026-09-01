// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/csharp"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the naming the language's own tooling discovers by", func(t *testing.T) {
			t.Parallel()
			assert.True(t, csharp.IsTest("src/StoreTests.cs"), "the test frameworks discover by this naming")
			assert.True(t, csharp.IsTest("Api.Tests/StoreFixture.cs"), "a test project holds tests")
			assert.False(t, csharp.IsTest("src/Store.cs"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown, because the name does not carry the rule", func(t *testing.T) {
			t.Parallel()
			// Reporting a guess would make a caller filtering to public
			// API drop declarations it should have kept.
			assert.Equal(t, csharp.Visibility("Store"), sema.VisibilityUnknown,
				"the modifier decides this, and reading the name would report everything as public")
			assert.Equal(t, csharp.Visibility("store"), sema.VisibilityUnknown,
				"the modifier decides this, and reading the name would report everything as public")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("drops the extension, leaving what an import names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, csharp.Namespace("src/Store.cs"), "src/Store",
				"two symbols in one file share a unit, and the extension is not part of its name")
		})
	})
}
