// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/conformance"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Suite", func(t *testing.T) {
		t.Parallel()

		t.Run("has no language when zero", func(t *testing.T) {
			t.Parallel()
			var zero conformance.Suite
			assert.Empty(t, string(zero.Declaration.Language), "language")
			assert.Empty(t, zero.Grammar.Tags, "tags")
			assert.Nil(t, zero.Grammar.Language, "grammar")
		})
	})
}
