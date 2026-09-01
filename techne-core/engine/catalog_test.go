// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// fake is an engine whose tier, price and availability a case sets. It
// serves RoleOutline, which is enough to be selected for.
type fake struct {
	name     string
	lang     source.Language
	fidelity trust.Fidelity
	cost     engine.Cost
	unusable error
}

func (f fake) Name() string                        { return f.name }
func (f fake) Language() source.Language           { return f.lang }
func (f fake) Fidelity(engine.Role) trust.Fidelity { return f.fidelity }
func (f fake) Cost(engine.Role) engine.Cost        { return f.cost }

func (fake) Outline(context.Context, engine.Request) (engine.Answer[sema.Symbol], error) {
	return engine.Answer[sema.Symbol]{Status: trust.OK}, nil
}

// gated is a fake that depends on something outside the process.
type gated struct{ fake }

func (g gated) Available(context.Context) error { return g.unusable }

func names(engines []engine.Engine) []string {
	out := make([]string, 0, len(engines))
	for _, e := range engines {
		out = append(out, e.Name())
	}
	return out
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestCatalog(t *testing.T) {
	t.Parallel()

	const lang = source.Language("fixture")

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses two engines with one name", func(t *testing.T) {
			t.Parallel()
			// Provenance names the engine that answered. Two engines
			// sharing a name make an answer untraceable.
			c := engine.NewCatalog()
			if err := c.Add(fake{name: "parser", lang: lang}); err != nil {
				t.Fatalf("first Add: %v", err)
			}
			if err := c.Add(fake{name: "parser", lang: lang}); err == nil {
				t.Error("a second engine with the same name was accepted")
			}
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("puts stronger evidence first", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: lang, fidelity: trust.Syntactic, cost: engine.CostParse})
			mustAdd(t, c, fake{name: "checker", lang: lang, fidelity: trust.Resolved, cost: engine.CostAnalyze})

			got := names(c.For(context.Background(), lang, engine.RoleOutline))
			if want := []string{"checker", "parser"}; !equal(got, want) {
				t.Errorf("order = %v, want %v", got, want)
			}
		})

		t.Run("puts the cheaper of two equals first", func(t *testing.T) {
			t.Parallel()
			// A warm index and the parser that filled it produce the
			// same facts. Nothing else separates them, so price does.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: lang, fidelity: trust.Syntactic, cost: engine.CostParse})
			mustAdd(t, c, fake{name: "index", lang: lang, fidelity: trust.Syntactic, cost: engine.CostMemory})

			got := names(c.For(context.Background(), lang, engine.RoleOutline))
			if want := []string{"index", "parser"}; !equal(got, want) {
				t.Errorf("order = %v, want %v", got, want)
			}
		})

		t.Run("skips an engine that does not serve the role", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "outliner", lang: lang})
			got := c.For(context.Background(), lang, engine.RoleVerify)
			if len(got) != 0 {
				t.Errorf("an engine with no Verify was selected: %v", names(got))
			}
		})

		t.Run("skips an engine that cannot run", func(t *testing.T) {
			t.Parallel()
			// An engine whose server is not installed must not be
			// advertised, or the ladder stops on something absent.
			c := engine.NewCatalog()
			mustAdd(t, c, gated{fake{
				name: "server", lang: lang, fidelity: trust.Resolved,
				unusable: errors.New("engine: not on PATH"),
			}})
			mustAdd(t, c, fake{name: "parser", lang: lang, fidelity: trust.Syntactic})

			got := names(c.For(context.Background(), lang, engine.RoleOutline))
			if want := []string{"parser"}; !equal(got, want) {
				t.Errorf("order = %v, want %v", got, want)
			}
		})

		t.Run("keeps an engine that declares no availability", func(t *testing.T) {
			t.Parallel()
			// An in-process engine has nothing outside to check. Making
			// it implement Available would mean every adapter carrying a
			// method that returns nil.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: lang})
			if got := c.For(context.Background(), lang, engine.RoleOutline); len(got) != 1 {
				t.Errorf("selected %v, want the one engine", names(got))
			}
		})

		t.Run("skips another language", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "other", lang: source.Language("elsewhere")})
			if got := c.For(context.Background(), lang, engine.RoleOutline); len(got) != 0 {
				t.Errorf("an engine for another language was selected: %v", names(got))
			}
		})
	})

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what each engine serves, and why it cannot", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, gated{fake{
				name: "server", lang: lang, fidelity: trust.Resolved,
				unusable: errors.New("engine: not on PATH"),
			}})

			var found bool
			for _, cap := range c.Capabilities(context.Background()) {
				if cap.Engine != "server" || cap.Role != engine.RoleOutline {
					continue
				}
				found = true
				if cap.Available {
					t.Error("an engine that cannot run was reported available")
				}
				if cap.Unavailable == "" {
					t.Error("an unavailable engine must say why")
				}
			}
			if !found {
				t.Error("the engine was not reported for the role it serves")
			}
		})

		t.Run("reports nothing for a role no engine serves", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "outliner", lang: lang})
			for _, cap := range c.Capabilities(context.Background()) {
				if cap.Role == engine.RoleVerify {
					t.Error("a role nothing serves was reported as a capability")
				}
			}
		})
	})
}

func mustAdd(t *testing.T, c *engine.Catalog, e engine.Engine) {
	t.Helper()
	if err := c.Add(e); err != nil {
		t.Fatalf("Add(%s): %v", e.Name(), err)
	}
}
