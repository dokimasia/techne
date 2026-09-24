// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// fixture is the language of the test declarations.
const fixture = source.Language("fixture")

// declared returns a complete declaration of fixture.
func declared() lang.Declaration {
	return lang.Declaration{
		Language:   fixture,
		Extensions: []string{".fx"},
		Manifests:  []string{"fixture.toml"},
		Comment:    lang.CommentStyle{Line: "// ", Doc: []lang.DocStyle{{Open: "//"}}},
		IsTest:     lang.JavaScriptTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

// stub is an engine named name that serves RoleOutline for language.
type stub struct {
	name     string
	language source.Language
}

func (s stub) Name() string                      { return s.name }
func (s stub) Language() source.Language         { return s.language }
func (stub) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (stub) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (stub) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("adds the engines to the catalogue", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared(), stub{"parser", fixture}), "Register")
			assert.Length(t, cat.For(t.Context(), fixture, engine.RoleOutline), 1, "engines")
		})

		incomplete := []struct {
			name  string
			spoil func(*lang.Declaration)
		}{
			{
				name:  "returns an error for a declaration without a language",
				spoil: func(d *lang.Declaration) { d.Language = "" },
			},
			{
				name:  "returns an error for a declaration without an extension",
				spoil: func(d *lang.Declaration) { d.Extensions = nil },
			},
			{
				name:  "returns an error for an extension without a leading dot",
				spoil: func(d *lang.Declaration) { d.Extensions = []string{"fx"} },
			},
			{
				name:  "returns an error for a declaration without IsTest",
				spoil: func(d *lang.Declaration) { d.IsTest = nil },
			},
			{
				name:  "returns an error for a declaration without Namespace",
				spoil: func(d *lang.Declaration) { d.Namespace = nil },
			},
			{
				name:  "returns an error for a declaration without Visibility",
				spoil: func(d *lang.Declaration) { d.Visibility = nil },
			},
		}
		for _, tt := range incomplete {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				d := declared()
				tt.spoil(&d)
				assert.HasError(t, lang.NewRegistry().Register(engine.NewCatalog(), d), "Register")
			})
		}

		t.Run("returns an error for a language already registered", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared()), "first Register")
			assert.HasError(t, r.Register(cat, declared()), "second Register")
		})

		t.Run("returns an error for an extension another language declares", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared()), "first Register")
			other := declared()
			other.Language = "other"
			assert.HasError(t, r.Register(cat, other), "second Register")
		})

		t.Run("returns an error for an engine of another language", func(t *testing.T) {
			t.Parallel()
			err := lang.NewRegistry().Register(engine.NewCatalog(), declared(), stub{"parser", "elsewhere"})
			assert.HasError(t, err, "Register")
		})

		t.Run("returns an error for two engines with one name", func(t *testing.T) {
			t.Parallel()
			err := lang.NewRegistry().Register(engine.NewCatalog(), declared(),
				stub{"parser", fixture}, stub{"parser", fixture})
			assert.HasError(t, err, "Register")
		})

		t.Run("leaves the catalogue unchanged when it returns an error", func(t *testing.T) {
			t.Parallel()
			cat := engine.NewCatalog()
			err := lang.NewRegistry().Register(cat, declared(), stub{"first", fixture}, stub{"first", fixture})
			assert.HasError(t, err, "Register")
			assert.Empty(t, cat.For(t.Context(), fixture, engine.RoleOutline), "engines")
		})

		t.Run("leaves the registry unchanged when it returns an error", func(t *testing.T) {
			t.Parallel()
			r := lang.NewRegistry()
			d := declared()
			d.Visibility = nil
			assert.HasError(t, r.Register(engine.NewCatalog(), d), "Register")
			_, ok := r.LanguageOf("a.fx")
			assert.False(t, ok, "LanguageOf")
		})
	})

	t.Run("LanguageOf", func(t *testing.T) {
		t.Parallel()

		r := lang.NewRegistry()
		assert.NoError(t, r.Register(engine.NewCatalog(), declared()), "Register")

		tests := []struct {
			name   string
			give   source.Path
			want   source.Language
			wantOK bool
		}{
			{name: "returns the language that declares the extension", give: "a/b/c.fx", want: fixture, wantOK: true},
			{name: "returns false for an undeclared extension", give: "a/b/c.unknown"},
			{name: "returns false for a path without an extension", give: "Makefile"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, ok := r.LanguageOf(tt.give)
				assert.Equal(t, ok, tt.wantOK, "ok")
				assert.Equal(t, got, tt.want, "language")
			})
		}
	})

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		r := lang.NewRegistry()
		assert.NoError(t, r.Register(engine.NewCatalog(), declared()), "Register")

		t.Run("returns the registered declaration", func(t *testing.T) {
			t.Parallel()
			got, ok := r.Declaration(fixture)
			assert.True(t, ok, "ok")
			assert.Equal(t, got.Extensions, []string{".fx"}, "Extensions")
		})

		t.Run("returns false for an unregistered language", func(t *testing.T) {
			t.Parallel()
			_, ok := r.Declaration("other")
			assert.False(t, ok, "ok")
		})
	})

	t.Run("Languages", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the languages sorted", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, r.Register(cat, declared()), "Register fixture")
			alpha := declared()
			alpha.Language, alpha.Extensions = "alpha", []string{".al"}
			assert.NoError(t, r.Register(cat, alpha), "Register alpha")
			assert.Equal(t, r.Languages(), []source.Language{"alpha", fixture}, "languages")
		})
	})
}
