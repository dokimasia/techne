// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/scala"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the naming the language's own tooling discovers by", func(t *testing.T) {
			t.Parallel()
			assert.True(t, scala.IsTest("src/test/scala/StoreSpec.scala"), "sbt and ScalaTest use this layout")
			assert.True(t, scala.IsTest("app/StoreTest.scala"), "sbt and ScalaTest use this naming")
			assert.False(t, scala.IsTest("src/main/scala/Store.scala"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown, because the name does not carry the rule", func(t *testing.T) {
			t.Parallel()
			// Reporting a guess would make a caller filtering to public
			// API drop declarations it should have kept.
			assert.Equal(t, scala.Visibility("Store"), sema.VisibilityUnknown,
				"the modifier decides this, and a name carries nothing of it")
			assert.Equal(t, scala.Visibility("store"), sema.VisibilityUnknown,
				"the modifier decides this, and a name carries nothing of it")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("drops the extension, leaving what an import names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Namespace("src/main/scala/Store.scala"), "src/main/scala/Store",
				"two symbols in one file share a unit, and the extension is not part of its name")
		})
	})
}
