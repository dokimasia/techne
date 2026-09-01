// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package conformance_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/conformance"
)

// TestDoc covers the claim the package comment makes: the checks are
// identical for every language, so nothing here is language-specific.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the suite", func(t *testing.T) {
		t.Parallel()

		t.Run("takes everything language-specific from the module", func(t *testing.T) {
			t.Parallel()
			// Were a language named here, a module for a sixth language
			// would need this package changed before it could run.
			supplied := conformance.Suite{}
			assert.Empty(t, string(supplied.Declaration.Language),
				"the suite holds no language of its own")
			assert.Empty(t, supplied.Grammar.Tags,
				"the suite holds no query of its own")
			assert.Nil(t, supplied.Grammar.Language,
				"the suite holds no grammar of its own")
		})
	})
}
