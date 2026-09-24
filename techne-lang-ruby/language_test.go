// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/ruby"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language ruby", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Declaration().Language, ruby.Language, "the language of the declaration")
			assert.Equal(t, string(ruby.Language), "ruby", "the value of Language")
		})

		t.Run("claims the extensions of Ruby", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Declaration().Extensions, []string{".rb", ".rake", ".gemspec"},
				"the extensions of Ruby")
		})

		t.Run("lists the manifests of a Ruby project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Declaration().Manifests, []string{
				"Gemfile", "gems.rb", "Rakefile", "rakefile", "Rakefile.rb", "rakefile.rb", "*.gemspec",
			}, "the manifests of Ruby")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Declaration().Namespace("lib/store.rb"), "lib/store", "the unit of lib/store.rb")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "store"} {
				assert.Equal(t, ruby.Declaration().Visibility(name), sema.VisibilityUnknown, "the visibility of "+name)
			}
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs ruby-lsp without arguments", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Server().Command, []string{"ruby-lsp"}, "the command of ruby-lsp")
		})

		t.Run("claims the indexed tier for relations", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Server().Fidelity(engine.RoleRelate), trust.Indexed, "the tier of RoleRelate")
		})

		t.Run("opens a file as ruby", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ruby.Server().LanguageID, lsp.IdentityRuby, "the language identifier of ruby-lsp")
		})
	})
}
