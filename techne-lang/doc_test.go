// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
)

// TestDoc covers the contract the package comment states across the
// declaration and the registry together.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("a language is a value the caller chooses", func(t *testing.T) {
		t.Parallel()

		t.Run("so one registry can hold a subset of another", func(t *testing.T) {
			t.Parallel()
			// Registration is an explicit call rather than an init with
			// a blank import. A test builds a registry holding one
			// language; a smaller binary ships a subset of the same
			// calls.
			full, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, full.Register(cat, declared(), stub{fixture}), "one language registers")

			empty := lang.NewRegistry()
			assert.Empty(t, empty.Languages(), "a registry nobody registered into holds nothing")
			assert.Length(t, full.Languages(), 1,
				"registration is a call, so each registry holds exactly what it was given")

			_, routed := empty.LanguageOf("a.fx")
			assert.False(t, routed, "a registry holding no language routes no path")
			assert.Length(t, cat.For(t.Context(), fixture, engine.RoleOutline), 1,
				"the engines went to the catalogue the caller supplied")
		})
	})
}
