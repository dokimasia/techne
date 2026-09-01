// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/query"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

const fixture = source.Language("fixture")

// router answers which language claims a path.
type router map[string]source.Language

func (r router) LanguageOf(p source.Path) (source.Language, bool) {
	l, claimed := r[string(p)]
	return l, claimed
}

// Languages is what a directory scope is asked of.
func (r router) Languages() []source.Language {
	out := make([]source.Language, 0, len(r))
	for _, l := range r {
		out = append(out, l)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// outliner is an engine whose tier, name and answer a case sets.
type outliner struct {
	name     string
	fidelity trust.Fidelity
	cost     engine.Cost
	found    []sema.Symbol
	err      error
}

func (o outliner) Name() string                        { return o.name }
func (outliner) Language() source.Language             { return fixture }
func (o outliner) Fidelity(engine.Role) trust.Fidelity { return o.fidelity }
func (o outliner) Cost(engine.Role) engine.Cost        { return o.cost }

func (o outliner) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	if o.err != nil {
		return engine.Result[sema.Symbol]{}, o.err
	}
	return engine.Result[sema.Symbol]{Items: o.found, Completeness: trust.ScopeTotal}, nil
}

func catalogue(t *testing.T, engines ...engine.Engine) *engine.Catalog {
	t.Helper()
	c := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, c.Add(e), "the case needs this engine registered")
	}
	return c
}

func symbol(name string) []sema.Symbol {
	return []sema.Symbol{{ID: sema.ID("fixture:.#" + name + ":function"), Name: name, Kind: sema.KindFunction}}
}

func TestService(t *testing.T) {
	t.Parallel()

	routes := router{"a.fx": fixture}

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("asks the engine holding the strongest evidence", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t,
				outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")},
				outliner{name: "checker", fidelity: trust.Resolved, found: symbol("strong")},
			)
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "an engine answered")
			assert.Equal(t, got.Provenance.Engine, "checker", "the strongest evidence is asked first")
			assert.Equal(t, got.Items[0].Name, "strong", "the answer is the one that engine gave")
		})

		t.Run("falls through when an engine declines this request", func(t *testing.T) {
			t.Parallel()
			// Declining is not failing: the engine serves the role but
			// cannot answer this one, so the next tier gets a turn.
			c := catalogue(t,
				outliner{name: "checker", fidelity: trust.Resolved, err: engine.ErrDecline},
				outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")},
			)
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "a weaker engine answered")
			assert.Equal(t, got.Provenance.Engine, "parser", "the next tier answers when the first declines")
		})

		t.Run("stops when an engine fails", func(t *testing.T) {
			t.Parallel()
			// Answering from a weaker engine would hide the breakage for
			// as long as anyone believed the answer.
			broken := errors.New("gopls: exit status 1")
			c := catalogue(t,
				outliner{name: "checker", fidelity: trust.Resolved, err: broken},
				outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")},
			)
			_, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.ErrorIs(t, err, broken, "a broken engine surfaces rather than being quietly replaced")
		})

		t.Run("reports unsupported when nothing serves the language", func(t *testing.T) {
			t.Parallel()
			got, err := query.New(engine.NewCatalog(), routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "having no engine is an answer, not a fault")
			assert.Equal(t, got.Status, trust.Unsupported, "nothing can answer this language and role")
			assert.False(t, got.Status.Answered(), "an unsupported answer carries no payload to read")
			assert.Empty(t, got.Items, "an unsupported answer carries no items")
		})

		t.Run("reports unsupported for a file no language claims", func(t *testing.T) {
			t.Parallel()
			// Guessing a language would answer about one nothing
			// declared, at a tier nothing earned.
			c := catalogue(t, outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")})
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "notes.md"})
			assert.NoError(t, err, "an unclaimed path is an answer, not a fault")
			assert.Equal(t, got.Status, trust.Unsupported, "no language claims this suffix")
			assert.NotEmpty(t, got.Provenance.Caveats,
				"a caller told only no cannot tell a gap from a mistake it could correct")
		})

		t.Run("asks every language about the workspace root", func(t *testing.T) {
			t.Parallel()
			// path.Ext(".") is ".", so the root reads as a file with an
			// extension unless it is handled. It is also the scope a
			// request naming none is given, so getting this wrong
			// answers nothing to the commonest call there is.
			c := catalogue(t, outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("found")})
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "."})
			assert.NoError(t, err, "the workspace root is a directory every language may claim part of")
			assert.Equal(t, got.Status, trust.OK, "the root is a directory, not a file with an extension")
			assert.NotEmpty(t, got.Items, "an engine serving the only registered language answered")
		})

		t.Run("takes the language the caller named over the path", func(t *testing.T) {
			t.Parallel()
			// A caller that knows the language should not need the
			// router to agree about a path it may not recognise.
			c := catalogue(t, outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")})
			got, err := query.New(c, router{}).Outline(t.Context(),
				engine.Request{Scope: "unrouted", Language: fixture})
			assert.NoError(t, err, "an engine answered")
			assert.Equal(t, got.Status, trust.OK, "the named language selected an engine without the router")
		})

		t.Run("marks an answer below the caller's floor as degraded", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, outliner{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")})
			got, err := query.New(c, routes).Outline(t.Context(),
				engine.Request{Scope: "a.fx", Preferred: trust.Resolved})
			assert.NoError(t, err, "a weaker engine still answered")
			assert.Equal(t, got.Status, trust.Degraded, "the caller asked for more than the answer holds")
			assert.True(t, got.Status.Answered(), "a degraded answer is still worth reading")
		})
	})
}
