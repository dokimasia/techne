// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/csharp"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("claims the wire form this module owns", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, string(csharp.Declaration().Language), "csharp",
				"the wire form reaches an index that outlives the process, so it is pinned here")
		})

		t.Run("states every convention the registry demands", func(t *testing.T) {
			t.Parallel()
			d := csharp.Declaration()
			assert.NotEmpty(t, d.Extensions, "without an extension nothing routes to this module")
			assert.NotNil(t, d.IsTest, "a nil convention panics on the first call")
			assert.NotNil(t, d.Namespace, "a nil convention panics on the first call")
			assert.NotNil(t, d.Visibility, "a nil convention panics on the first call")
			assert.NotEmpty(t, d.Comment.Line,
				"the document operations need a comment prefix no grammar states")
			assert.NotEmpty(t, d.Comment.Doc,
				"a language states the forms its own documentation tool reads")
		})

		t.Run("claims .cs", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, csharp.Declaration().Extensions, ".cs",
				"a file with this suffix is this language's to answer about")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("puts the language in the registry and its engine in the catalogue", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, csharp.Register(fstest.MapFS{}, r, c),
				"a composition root registers this module with one call")
			assert.Length(t, r.Languages(), 1, "one call registers one language")
			assert.Length(t, c.For(t.Context(), csharp.Declaration().Language, engine.RoleOutline), 1,
				"the parser is selectable for the role it serves")
		})

		t.Run("refuses a second registration of one language", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, csharp.Register(fstest.MapFS{}, r, c), "the first call registers")
			assert.HasError(t, csharp.Register(fstest.MapFS{}, r, c),
				"two claims on one language would make routing depend on call order")
		})
	})
}
