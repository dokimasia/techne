// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"context"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

const fixture = source.Language("fixture")

// declared returns a complete declaration a case can spoil one field of.
func declared() lang.Declaration {
	return lang.Declaration{
		Language:   fixture,
		Extensions: []string{".fx"},
		Manifests:  []string{"fixture.toml"},
		Comment:    lang.CommentStyle{Line: "// ", Above: true},
		IsTest:     func(p string) bool { return strings.HasSuffix(p, "_test.fx") },
		Namespace:  func(p string) string { return strings.TrimSuffix(p, ".fx") },
		Visibility: visibility,
	}
}

// stub serves one role for one language.
type stub struct{ lang source.Language }

func (stub) Name() string                        { return "stub" }
func (s stub) Language() source.Language         { return s.lang }
func (stub) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (stub) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (stub) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("adds every engine to the catalogue", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared(), stub{fixture}), "a complete declaration registers")
			assert.Length(t, cat.For(t.Context(), fixture, engine.RoleOutline), 1,
				"registering a language puts its engines where the catalogue can select them")
		})

		t.Run("refuses a declaration naming no language", func(t *testing.T) {
			t.Parallel()
			d := declared()
			d.Language = ""
			assert.HasError(t, lang.NewRegistry().Register(engine.NewCatalog(), d),
				"a declaration naming no language claims nothing to route to")
		})

		t.Run("refuses a declaration no file can route to", func(t *testing.T) {
			t.Parallel()
			// Without an extension nothing selects the language, so the
			// module would register and never answer.
			d := declared()
			d.Extensions = nil
			assert.HasError(t, lang.NewRegistry().Register(engine.NewCatalog(), d),
				"without an extension nothing selects the language, so it would register and never answer")
		})

		t.Run("refuses a declaration missing a convention", func(t *testing.T) {
			t.Parallel()
			// A nil func panics at the first call. Startup is where a
			// language module's mistake should surface.
			for name, spoil := range map[string]func(*lang.Declaration){
				"IsTest":     func(d *lang.Declaration) { d.IsTest = nil },
				"Namespace":  func(d *lang.Declaration) { d.Namespace = nil },
				"Visibility": func(d *lang.Declaration) { d.Visibility = nil },
			} {
				d := declared()
				spoil(&d)
				_ = name
				assert.HasError(t, lang.NewRegistry().Register(engine.NewCatalog(), d),
					"a nil convention panics on the first call, so startup is where it must surface")
			}
		})

		t.Run("refuses a second claim on one language", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared()), "the first module claims the language")
			assert.HasError(t, r.Register(cat, declared()),
				"two modules claiming one language would make routing depend on registration order")
		})

		t.Run("refuses a second claim on one extension", func(t *testing.T) {
			t.Parallel()
			// Two languages claiming one suffix makes routing depend on
			// registration order, which nothing states.
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared()), "the first module claims the extension")
			other := declared()
			other.Language = source.Language("other")
			assert.HasError(t, r.Register(cat, other),
				"two languages claiming one suffix would make routing depend on registration order")
		})

		t.Run("refuses an engine for a different language", func(t *testing.T) {
			t.Parallel()
			// The declaration and its engines have to agree, or the
			// catalogue holds an engine no path routes to.
			err := lang.NewRegistry().Register(engine.NewCatalog(), declared(), stub{source.Language("elsewhere")})
			assert.HasError(t, err,
				"a declaration and its engines must agree, or the catalogue holds an engine no path routes to")
		})

		t.Run("leaves the catalogue untouched when it refuses", func(t *testing.T) {
			t.Parallel()
			// A partial registration would leave engines behind for a
			// language nothing can route to.
			cat := engine.NewCatalog()
			d := declared()
			d.Extensions = nil
			_ = lang.NewRegistry().Register(cat, d, stub{fixture})
			assert.Empty(t, cat.For(t.Context(), fixture, engine.RoleOutline),
				"a rejected module leaves no engines behind for a language nothing can route to")
		})
	})

	t.Run("Languages", func(t *testing.T) {
		t.Parallel()

		t.Run("answers in a stable order", func(t *testing.T) {
			t.Parallel()
			// A service asking every language about a directory merges
			// their answers. Map iteration would reorder the result
			// between two identical requests.
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared()), "the case needs a language registered")

			other := declared()
			other.Language = source.Language("alpha")
			other.Extensions = []string{".al"}
			assert.NoError(t, r.Register(cat, other), "the case needs a second language registered")

			assert.Equal(t, r.Languages(), []source.Language{"alpha", "fixture"},
				"two identical requests see the same order")
		})
	})

	t.Run("LanguageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("routes a path by its extension", func(t *testing.T) {
			t.Parallel()
			r := lang.NewRegistry()
			assert.NoError(t, r.Register(engine.NewCatalog(), declared()), "the case needs the language registered")
			got, routed := r.LanguageOf("a/b/c.fx")
			assert.True(t, routed, "a path whose suffix a language claims routes to it")
			assert.Equal(t, got, fixture, "a path whose suffix a language claims routes to it")
		})

		t.Run("reports nothing for an unclaimed extension", func(t *testing.T) {
			t.Parallel()
			// Guessing here would answer about a language nothing
			// declared, at a fidelity nothing earned.
			r := lang.NewRegistry()
			assert.NoError(t, r.Register(engine.NewCatalog(), declared()), "the case needs the language registered")
			_, routed := r.LanguageOf("a/b/c.unclaimed")
			assert.False(t, routed, "guessing would answer about a language nothing declared")
		})

		t.Run("reports nothing for a path with no extension", func(t *testing.T) {
			t.Parallel()
			r := lang.NewRegistry()
			assert.NoError(t, r.Register(engine.NewCatalog(), declared()), "the case needs the language registered")
			_, routed := r.LanguageOf("Makefile")
			assert.False(t, routed, "a path carrying no suffix names no language")
		})
	})
}

// visibility is the fixture language's rule: a capitalised name is
// visible outside its unit.
func visibility(n string) sema.Visibility {
	if n != "" && n[0] >= 'A' && n[0] <= 'Z' {
		return sema.Exported
	}
	return sema.Unexported
}
