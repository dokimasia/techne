// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
)

func TestDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("is refused by Register when zero", func(t *testing.T) {
			t.Parallel()
			var zero lang.Declaration
			assert.HasError(t, lang.NewRegistry().Register(engine.NewCatalog(), zero), "Register")
		})

		t.Run("is accepted by Register when complete", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, lang.NewRegistry().Register(engine.NewCatalog(), declared()), "Register")
		})
	})
}
