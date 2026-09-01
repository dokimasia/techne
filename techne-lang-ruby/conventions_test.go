// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/ruby"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the naming the language's own tooling discovers by", func(t *testing.T) {
			t.Parallel()
			assert.True(t, ruby.IsTest("spec/store_spec.rb"), "RSpec and Minitest discover by this naming")
			assert.True(t, ruby.IsTest("test/store_test.rb"), "RSpec and Minitest discover by this naming")
			assert.False(t, ruby.IsTest("lib/store.rb"), "shipped code is not a test")
		})
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown, because the name does not carry the rule", func(t *testing.T) {
			t.Parallel()
			// Reporting a guess would make a caller filtering to public
			// API drop declarations it should have kept.
			assert.Equal(t, ruby.Visibility("Store"), sema.VisibilityUnknown,
				"a private call before the declaration decides this, and a name carries nothing of it")
			assert.Equal(t, ruby.Visibility("store"), sema.VisibilityUnknown,
				"a private call before the declaration decides this, and a name carries nothing of it")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("drops the extension, leaving what an import names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Namespace("lib/store.rb"), "lib/store",
				"two symbols in one file share a unit, and the extension is not part of its name")
		})
	})
}
