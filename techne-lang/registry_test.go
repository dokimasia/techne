// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"context"
	"strings"
	"testing"

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
		Exported:   func(n string) bool { return n != "" && n[0] >= 'A' && n[0] <= 'Z' },
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
			if err := r.Register(cat, declared(), stub{fixture}); err != nil {
				t.Fatalf("Register: %v", err)
			}
			if got := cat.For(context.Background(), fixture, engine.RoleOutline); len(got) != 1 {
				t.Errorf("catalogue holds %d engines for the language, want 1", len(got))
			}
		})

		t.Run("refuses a declaration naming no language", func(t *testing.T) {
			t.Parallel()
			d := declared()
			d.Language = ""
			if err := lang.NewRegistry().Register(engine.NewCatalog(), d); err == nil {
				t.Error("a declaration with no language was accepted")
			}
		})

		t.Run("refuses a declaration no file can route to", func(t *testing.T) {
			t.Parallel()
			// Without an extension nothing selects the language, so the
			// module would register and never answer.
			d := declared()
			d.Extensions = nil
			if err := lang.NewRegistry().Register(engine.NewCatalog(), d); err == nil {
				t.Error("a declaration with no extension was accepted")
			}
		})

		t.Run("refuses a declaration missing a convention", func(t *testing.T) {
			t.Parallel()
			// A nil func panics at the first call. Startup is where a
			// language module's mistake should surface.
			for name, spoil := range map[string]func(*lang.Declaration){
				"IsTest":    func(d *lang.Declaration) { d.IsTest = nil },
				"Namespace": func(d *lang.Declaration) { d.Namespace = nil },
				"Exported":  func(d *lang.Declaration) { d.Exported = nil },
			} {
				d := declared()
				spoil(&d)
				if err := lang.NewRegistry().Register(engine.NewCatalog(), d); err == nil {
					t.Errorf("a declaration with no %s was accepted", name)
				}
			}
		})

		t.Run("refuses a second claim on one language", func(t *testing.T) {
			t.Parallel()
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			if err := r.Register(cat, declared()); err != nil {
				t.Fatalf("first Register: %v", err)
			}
			if err := r.Register(cat, declared()); err == nil {
				t.Error("a second declaration for one language was accepted")
			}
		})

		t.Run("refuses a second claim on one extension", func(t *testing.T) {
			t.Parallel()
			// Two languages claiming one suffix makes routing depend on
			// registration order, which nothing states.
			r, cat := lang.NewRegistry(), engine.NewCatalog()
			if err := r.Register(cat, declared()); err != nil {
				t.Fatalf("first Register: %v", err)
			}
			other := declared()
			other.Language = source.Language("other")
			if err := r.Register(cat, other); err == nil {
				t.Error("a second language claiming the same extension was accepted")
			}
		})

		t.Run("refuses an engine for a different language", func(t *testing.T) {
			t.Parallel()
			// The declaration and its engines have to agree, or the
			// catalogue holds an engine no path routes to.
			err := lang.NewRegistry().Register(engine.NewCatalog(), declared(), stub{source.Language("elsewhere")})
			if err == nil {
				t.Error("an engine for another language was accepted")
			}
		})

		t.Run("leaves the catalogue untouched when it refuses", func(t *testing.T) {
			t.Parallel()
			// A partial registration would leave engines behind for a
			// language nothing can route to.
			cat := engine.NewCatalog()
			d := declared()
			d.Extensions = nil
			_ = lang.NewRegistry().Register(cat, d, stub{fixture})
			if got := cat.For(context.Background(), fixture, engine.RoleOutline); len(got) != 0 {
				t.Errorf("a refused registration left %d engines behind", len(got))
			}
		})
	})

	t.Run("LanguageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("routes a path by its extension", func(t *testing.T) {
			t.Parallel()
			r := lang.NewRegistry()
			if err := r.Register(engine.NewCatalog(), declared()); err != nil {
				t.Fatalf("Register: %v", err)
			}
			got, ok := r.LanguageOf("a/b/c.fx")
			if !ok || got != fixture {
				t.Errorf("LanguageOf = %q, %v; want %q, true", got, ok, fixture)
			}
		})

		t.Run("reports nothing for an unclaimed extension", func(t *testing.T) {
			t.Parallel()
			// Guessing here would answer about a language nothing
			// declared, at a fidelity nothing earned.
			r := lang.NewRegistry()
			if err := r.Register(engine.NewCatalog(), declared()); err != nil {
				t.Fatalf("Register: %v", err)
			}
			if got, ok := r.LanguageOf("a/b/c.unclaimed"); ok {
				t.Errorf("LanguageOf on an unclaimed suffix = %q, want no language", got)
			}
		})

		t.Run("reports nothing for a path with no extension", func(t *testing.T) {
			t.Parallel()
			r := lang.NewRegistry()
			if err := r.Register(engine.NewCatalog(), declared()); err != nil {
				t.Fatalf("Register: %v", err)
			}
			if _, ok := r.LanguageOf("Makefile"); ok {
				t.Error("a path with no extension routed to a language")
			}
		})
	})
}
