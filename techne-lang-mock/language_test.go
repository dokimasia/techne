// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/mock"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name as the language and the extension", func(t *testing.T) {
			t.Parallel()
			held := mock.Declaration("alpha")
			assert.Equal(t, held.Language, source.Language("alpha"), "the language")
			assert.Equal(t, held.Extensions, []string{".alpha"}, "the extensions")
		})
	})

	t.Run("Registering", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for an empty name", func(t *testing.T) {
			t.Parallel()
			err := mock.Registering("")(lang.Workspace{FS: fstest.MapFS{}}, lang.NewRegistry(), engine.NewCatalog())
			assert.HasError(t, err, "Registering without a name")
		})

		t.Run("registers two languages that route apart", func(t *testing.T) {
			t.Parallel()
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			for _, name := range []string{"alpha", "beta"} {
				assert.NoError(t, mock.Registering(name)(lang.Workspace{FS: fstest.MapFS{}}, registry, catalogue),
					"Registering "+name)
			}
			for _, one := range []struct {
				path source.Path
				want source.Language
			}{{"a.alpha", "alpha"}, {"b.beta", "beta"}} {
				got, claimed := registry.LanguageOf(one.path)
				assert.True(t, claimed, "the claim of "+string(one.path))
				assert.Equal(t, got, one.want, "the language of "+string(one.path))
			}
		})

		t.Run("registers two languages at different tiers", func(t *testing.T) {
			t.Parallel()
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			w := lang.Workspace{FS: fstest.MapFS{}}
			assert.NoError(t, mock.Registering("strong")(w, registry, catalogue), "Registering strong")
			assert.NoError(t, mock.Registering("weak", mock.At(trust.Syntactic))(w, registry, catalogue),
				"Registering weak")
			for _, one := range []struct {
				language source.Language
				want     trust.Fidelity
			}{{"strong", trust.Resolved}, {"weak", trust.Syntactic}} {
				held := catalogue.For(t.Context(), one.language, engine.RolePlan)
				assert.Length(t, held, 1, "the planners of "+string(one.language))
				assert.Equal(t, held[0].Fidelity(engine.RolePlan), one.want, "the tier of "+string(one.language))
			}
		})
	})

	t.Run("Covering", func(t *testing.T) {
		t.Parallel()

		t.Run("sets the completeness of an answer", func(t *testing.T) {
			t.Parallel()
			got, err := built(t, mock.Covering(trust.ScopePartial)).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "Outline")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the outline")
		})
	})

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a file whose name ends in _test", func(t *testing.T) {
			t.Parallel()
			assert.True(t, mock.IsTest("a/b_test.mock"), "IsTest of a/b_test.mock")
		})

		t.Run("reports false for a file without the suffix", func(t *testing.T) {
			t.Parallel()
			assert.False(t, mock.IsTest("a/b.mock"), "IsTest of a/b.mock")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the directory of a file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mock.Namespace("src/a.mock"), "src", "the namespace of src/a.mock")
		})

		t.Run("returns the empty string for a file at the root", func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, mock.Namespace("a.mock"), "the namespace of a.mock")
		})
	})
}
