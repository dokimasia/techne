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

func TestRegistering(t *testing.T) {
	t.Parallel()

	t.Run("a name", func(t *testing.T) {
		t.Parallel()

		t.Run("is the language and its extension both", func(t *testing.T) {
			t.Parallel()
			held := mock.Declaration("alpha")
			assert.Equal(t, held.Language, source.Language("alpha"), "the language is what it is called")
			assert.Equal(t, held.Extensions, []string{".alpha"}, "and it claims the extension of that name")
		})

		t.Run("is refused when there is none", func(t *testing.T) {
			t.Parallel()
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			assert.HasError(t, mock.Registering("")(fstest.MapFS{}, registry, catalogue),
				"a language without a name routes nowhere")
		})
	})

	t.Run("several languages", func(t *testing.T) {
		t.Parallel()

		t.Run("register side by side and route separately", func(t *testing.T) {
			t.Parallel()
			// This is what the module is for. A workspace holding two
			// languages is what a merged answer, a per-language refusal
			// and routing by extension need in order to be exercised at
			// all, and nothing techne ships can stand one up.
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			for _, name := range []string{"alpha", "beta"} {
				assert.NoError(t, mock.Registering(name)(fstest.MapFS{}, registry, catalogue),
					"each language registers")
			}

			assert.Length(t, registry.Languages(), 2, "both are known")
			for _, one := range []struct {
				path source.Path
				want source.Language
			}{{"a.alpha", "alpha"}, {"b.beta", "beta"}} {
				got, claimed := registry.LanguageOf(one.path)
				assert.True(t, claimed, "each extension routes")
				assert.Equal(t, got, one.want, "to the language whose name it is")
			}
		})

		t.Run("can claim different tiers, so a refusal is real", func(t *testing.T) {
			t.Parallel()
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t,
				mock.Registering("strong")(fstest.MapFS{}, registry, catalogue),
				"one that resolves registers")
			assert.NoError(t,
				mock.Registering("weak", mock.At(trust.Syntactic))(fstest.MapFS{}, registry, catalogue),
				"and one that only parses registers beside it")

			for _, one := range []struct {
				language source.Language
				want     trust.Fidelity
			}{{"strong", trust.Resolved}, {"weak", trust.Syntactic}} {
				held := catalogue.For(t.Context(), one.language, engine.RolePlan)
				assert.Length(t, held, 1, "each language has its engine")
				assert.Equal(t, held[0].Fidelity(engine.RolePlan), one.want,
					"claiming what it was registered to claim")
			}
		})
	})

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		t.Run("is the suffix this language spells it with", func(t *testing.T) {
			t.Parallel()
			assert.True(t, mock.IsTest("a/b_test.mock"), "a test file is named one")
			assert.False(t, mock.IsTest("a/b.mock"), "and shipped code is not")
		})
	})

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("is the directory, which is all the module system there is", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mock.Namespace("src/a.mock"), "src", "a file belongs to its directory")
			assert.Empty(t, mock.Namespace("a.mock"), "and one at the root belongs to none")
		})
	})
}
