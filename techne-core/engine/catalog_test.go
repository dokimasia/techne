// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
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

func (fake) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
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

func TestCatalog(t *testing.T) {
	t.Parallel()

	const lang = source.Language("fixture")

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses two engines with one name for one language", func(t *testing.T) {
			t.Parallel()
			// Provenance names the engine that answered. Two engines
			// sharing a name make an answer untraceable.
			c := engine.NewCatalog()
			assert.NoError(t, c.Add(fake{name: "parser", lang: lang}), "the first engine registers")
			assert.HasError(t, c.Add(fake{name: "parser", lang: lang}),
				"a provenance names the engine that answered, so two engines cannot share a name")
		})

		t.Run("accepts one name answering about two languages", func(t *testing.T) {
			t.Parallel()
			// One program serves several: typescript-language-server
			// answers about TypeScript and about JavaScript, clangd about
			// C and C++. Refusing the second would make a language lose
			// its server for being second in the list.
			c := engine.NewCatalog()
			assert.NoError(t, c.Add(fake{name: "shared", lang: lang}), "the first registers")
			assert.NoError(t, c.Add(fake{name: "shared", lang: source.Language("elsewhere")}),
				"and so does the same program answering about another language")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("puts stronger evidence first", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: lang, fidelity: trust.Syntactic, cost: engine.CostParse})
			mustAdd(t, c, fake{name: "checker", lang: lang, fidelity: trust.Resolved, cost: engine.CostAnalyze})

			assert.Equal(t, names(c.For(t.Context(), lang, engine.RoleOutline)),
				[]string{"checker", "parser"},
				"the strongest evidence is tried first")
		})

		t.Run("puts the cheaper of two equals first", func(t *testing.T) {
			t.Parallel()
			// A warm index and the parser that filled it produce the
			// same facts. Nothing else separates them, so price does.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: lang, fidelity: trust.Syntactic, cost: engine.CostParse})
			mustAdd(t, c, fake{name: "index", lang: lang, fidelity: trust.Syntactic, cost: engine.CostMemory})

			assert.Equal(t, names(c.For(t.Context(), lang, engine.RoleOutline)),
				[]string{"index", "parser"},
				"two engines producing the same facts are separated by price alone")
		})

		t.Run("skips an engine that does not serve the role", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "outliner", lang: lang, fidelity: trust.Syntactic})
			assert.Empty(t, c.For(t.Context(), lang, engine.RoleVerify),
				"an engine lacking the method does not serve the role")
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

			assert.Equal(t, names(c.For(t.Context(), lang, engine.RoleOutline)),
				[]string{"parser"},
				"an engine whose server is absent is not advertised")
		})

		t.Run("keeps an engine that declares no availability", func(t *testing.T) {
			t.Parallel()
			// An in-process engine has nothing outside to check. Making
			// it implement Available would mean every adapter carrying a
			// method that returns nil.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: lang, fidelity: trust.Syntactic})
			assert.Length(t, c.For(t.Context(), lang, engine.RoleOutline), 1,
				"an in-process engine has nothing outside to check and is always usable")
		})

		t.Run("skips an engine that reaches nothing for the role", func(t *testing.T) {
			t.Parallel()
			// An engine implements a port for the roles it serves and
			// cannot implement it for some and not others, so declaring
			// no evidence is how it declines the rest. Offered anyway, it
			// would be tried when the engine above it declines, and would
			// appear in a capability report as serving at no tier.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "silent", lang: lang, fidelity: trust.None})
			assert.Empty(t, c.For(t.Context(), lang, engine.RoleOutline),
				"an engine holding no evidence for a role has nothing to say about it")
		})

		t.Run("skips another language", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{
				name: "other", lang: source.Language("elsewhere"),
				fidelity: trust.Syntactic,
			})
			assert.Empty(t, c.For(t.Context(), lang, engine.RoleOutline),
				"an engine answers about one language and is not offered for another")
		})
	})

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no role an engine reaches nothing for", func(t *testing.T) {
			t.Parallel()
			// The report and the selection ask the same question. One
			// advertising a role the other will never select tells a
			// caller it can do something it cannot.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "silent", lang: lang, fidelity: trust.None})
			assert.Empty(t, c.Capabilities(t.Context()),
				"nothing is advertised that nothing can be selected for")
		})

		t.Run("reports what each engine serves, and why it cannot", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, gated{fake{
				name: "server", lang: lang, fidelity: trust.Resolved,
				unusable: errors.New("engine: not on PATH"),
			}})

			var found bool
			for _, cap := range c.Capabilities(t.Context()) {
				if cap.Engine != "server" || cap.Role != engine.RoleOutline {
					continue
				}
				found = true
				assert.False(t, cap.Available, "an engine whose server is absent cannot run")
				assert.NotEmpty(t, cap.Unavailable,
					"a missing server is a different problem from a missing capability, so it says which")
			}
			assert.True(t, found, "an engine is reported for every role it serves")
		})

		t.Run("reports nothing for a role no engine serves", func(t *testing.T) {
			t.Parallel()
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "outliner", lang: lang})
			for _, cap := range c.Capabilities(t.Context()) {
				assert.NotEqual(t, cap.Role, engine.RoleVerify,
					"a role no engine serves is not reported as a capability")
			}
		})
	})
}

func mustAdd(t *testing.T, c *engine.Catalog, e engine.Engine) {
	t.Helper()
	assert.NoError(t, c.Add(e), "the case needs this engine registered")
}
