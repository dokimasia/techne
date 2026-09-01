// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/c"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the naming the language's own tooling discovers by", func(t *testing.T) {
			t.Parallel()
			assert.True(t, c.IsTest("tests/parser_test.c"), "the common frameworks discover by this naming")
			assert.True(t, c.IsTest("src/test_parser.c"), "the common frameworks discover by this naming")
			assert.False(t, c.IsTest("src/parser.c"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown, because the name does not carry the rule", func(t *testing.T) {
			t.Parallel()
			// Reporting a guess would make a caller filtering to public
			// API drop declarations it should have kept.
			assert.Equal(t, c.Visibility("Store"), sema.VisibilityUnknown,
				"static and the header decide this, and a name carries neither")
			assert.Equal(t, c.Visibility("store"), sema.VisibilityUnknown,
				"static and the header decide this, and a name carries neither")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("drops the extension, leaving what an import names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.Namespace("src/parser.c"), "src/parser",
				"two symbols in one file share a unit, and the extension is not part of its name")
		})
	})
}
