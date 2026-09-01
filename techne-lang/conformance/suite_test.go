// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
)

func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing, so a module cannot pass by supplying nothing", func(t *testing.T) {
			t.Parallel()
			var unset conformance.Suite
			assert.Empty(t, unset.Files, "a suite with no source outlines nothing")
			assert.Empty(t, unset.Declares, "a suite expecting nothing asserts nothing")
			assert.Empty(t, string(unset.Declaration.Language),
				"a suite carrying no declaration cannot register")
		})
	})

	t.Run("Declared", func(t *testing.T) {
		t.Parallel()

		t.Run("expects unknown visibility by default", func(t *testing.T) {
			t.Parallel()
			// Three of the five languages spell visibility as a modifier
			// a tags query does not capture, so unknown is the common
			// case and the zero value matches it.
			var unset conformance.Declared
			assert.Equal(t, unset.Visibility, sema.VisibilityUnknown,
				"a module that says nothing about visibility expects the answer a parser can give")
		})
	})
}
