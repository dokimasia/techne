// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/java"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the JUnit naming and the build layout", func(t *testing.T) {
			t.Parallel()
			assert.True(t, java.IsTest("src/test/java/pkg/Store.java"), "the Maven and Gradle test tree")
			assert.True(t, java.IsTest("pkg/StoreTest.java"), "the JUnit suffix")
			assert.True(t, java.IsTest("pkg/StoreTests.java"), "the JUnit plural suffix")
			assert.False(t, java.IsTest("src/main/java/pkg/Store.java"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is always unknown, because a modifier carries it", func(t *testing.T) {
			t.Parallel()
			// Reading the name would report every declaration as public,
			// which is the claim this tier cannot support.
			for _, name := range []string{"Store", "helper", "PUBLIC_CONSTANT", ""} {
				assert.Equal(t, java.Visibility(name), sema.VisibilityUnknown,
					"a tags query captures the name, and Java spells visibility as a modifier")
			}
		})
	})
}
