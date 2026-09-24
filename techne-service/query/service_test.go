// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/query"
)

const fixture = source.Language("fixture")

// router maps each path of the map to its language, and lists the languages of the map.
type router map[source.Path]source.Language

func (r router) LanguageOf(p source.Path) (source.Language, bool) {
	l, claimed := r[p]
	return l, claimed
}

func (r router) Languages() []source.Language {
	return slices.Compact(slices.Sorted(maps.Values(r)))
}

// answering is an engine of one language, fixture by default. Its outline returns found
// with the completeness, the caveats and the skip that a case sets, or err. Each other
// read port returns one item named after its role.
type answering struct {
	name     string
	language source.Language
	fidelity trust.Fidelity
	coverage trust.Completeness
	found    []sema.Symbol
	caveats  []trust.Caveat
	skipped  bool
	err      error
}

func (a answering) Name() string                        { return a.name }
func (a answering) Language() source.Language           { return cmp.Or(a.language, fixture) }
func (a answering) Fidelity(engine.Role) trust.Fidelity { return a.fidelity }
func (answering) Cost(engine.Role) engine.Cost          { return engine.CostParse }

func (a answering) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	if a.err != nil {
		return engine.Result[sema.Symbol]{}, a.err
	}
	return engine.Result[sema.Symbol]{
		Items:        a.found,
		Completeness: cmp.Or(a.coverage, trust.ScopeTotal),
		Caveats:      a.caveats,
		Skipped:      a.skipped,
	}, nil
}

func (answering) Search(context.Context, engine.Request, engine.Query) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Items: symbol("search"), Completeness: trust.ScopeTotal}, nil
}

func (answering) Resolve(context.Context, engine.Request, source.Position) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Items: symbol("resolve"), Completeness: trust.ScopeTotal}, nil
}

func (answering) Relate(
	context.Context,
	engine.Request,
	sema.ID,
	sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	return engine.Result[sema.Relation]{Items: []sema.Relation{{Via: "relate"}}, Completeness: trust.ScopeTotal}, nil
}

func (answering) Verify(context.Context, engine.Request, []string) (engine.Result[edit.Finding], error) {
	return engine.Result[edit.Finding]{
		Items:        []edit.Finding{{Diagnostic: diag.Diagnostic{Message: "verify"}}},
		Completeness: trust.ScopeTotal,
	}, nil
}

// catalogue returns a catalogue of engines.
func catalogue(t *testing.T, engines ...engine.Engine) *engine.Catalog {
	t.Helper()
	c := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, c.Add(e), "Add of "+e.Name())
	}
	return c
}

// symbol returns one function named name.
func symbol(name string) []sema.Symbol {
	return []sema.Symbol{{ID: sema.NewID(fixture, ".", name, sema.KindFunction), Name: name, Kind: sema.KindFunction}}
}

// declining returns an error that wraps engine.ErrDecline with why.
func declining(why string) error { return fmt.Errorf("%w: %s", engine.ErrDecline, why) }

func TestService(t *testing.T) {
	t.Parallel()

	routes := router{"a.fx": fixture}

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("asks the engine of the strongest tier first", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t,
				answering{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")},
				answering{name: "checker", fidelity: trust.Resolved, found: symbol("strong")},
			)
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Provenance.Engine, "checker", "the engine of the answer")
			assert.Equal(t, got.Items, symbol("strong"), "the declarations of the answer")
		})

		t.Run("asks the next engine after a decline", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t,
				answering{name: "checker", fidelity: trust.Resolved, err: declining("the server is starting")},
				answering{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")},
			)
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Provenance.Engine, "parser", "the engine of the answer")
		})

		t.Run("returns the error of a failing engine", func(t *testing.T) {
			t.Parallel()
			broken := errors.New("gopls: exit status 1")
			c := catalogue(t,
				answering{name: "checker", fidelity: trust.Resolved, err: broken},
				answering{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")},
			)
			_, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.ErrorIs(t, err, broken, "the error of Outline")
		})

		t.Run("returns unsupported for a language without an engine", func(t *testing.T) {
			t.Parallel()
			got, err := query.New(engine.NewCatalog(), routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the answer")
			assert.Empty(t, got.Items, "the declarations of the answer")
		})

		t.Run("returns unsupported with the reason when the only engine declines", func(t *testing.T) {
			t.Parallel()
			starting := answering{name: "checker", fidelity: trust.Resolved, err: declining("the server is starting")}
			got, err := query.New(catalogue(t, starting), routes).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the answer")
			assert.Equal(t, got.Provenance.Caveats, []trust.Caveat{{
				Code: trust.CaveatUnsupported,
				Note: "checker: the server is starting",
			}}, "the caveats of the answer")
		})

		t.Run("returns unsupported with the reason when every answer is skipped and one declines", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t,
				answering{name: "checker", fidelity: trust.Resolved, err: declining("src is a directory")},
				answering{name: "server", language: other, fidelity: trust.Resolved, skipped: true},
			)
			got, err := query.New(c, suffixes{fixture, other}).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "Outline of src")
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the answer")
			assert.Empty(t, got.Provenance.Engine, "the engines of the answer")
			assert.Equal(t, got.Provenance.Caveats, []trust.Caveat{{
				Code: trust.CaveatUnsupported,
				Note: "checker: src is a directory",
			}}, "the caveats of the answer")
		})

		t.Run("returns unsupported with a caveat for a path that no language claims", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")})
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: "notes.md"})
			assert.NoError(t, err, "Outline of notes.md")
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the answer")
			assert.NotEmpty(t, got.Provenance.Caveats, "the caveats of the answer")
		})

		t.Run("asks every language about the workspace root", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic, found: symbol("found")})
			got, err := query.New(c, routes).Outline(t.Context(), engine.Request{Scope: engine.Root})
			assert.NoError(t, err, "Outline of the root")
			assert.Equal(t, got.Status, trust.OK, "the status of the answer")
			assert.Equal(t, got.Items, symbol("found"), "the declarations of the answer")
		})

		t.Run("asks the language that the request names", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")})
			got, err := query.New(c, router{}).Outline(t.Context(),
				engine.Request{Scope: "unrouted", Language: fixture})
			assert.NoError(t, err, "Outline of unrouted")
			assert.Equal(t, got.Status, trust.OK, "the status of the answer")
		})

		t.Run("returns a degraded answer below the preferred tier", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic, found: symbol("weak")})
			got, err := query.New(c, routes).Outline(t.Context(),
				engine.Request{Scope: "a.fx", Preferred: trust.Resolved})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Status, trust.Degraded, "the status of the answer")
		})
	})

	t.Run("Search", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of the search port", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic})
			got, err := query.New(c, routes).Search(t.Context(), engine.Request{Scope: "a.fx"}, engine.Query{Text: "x"})
			assert.NoError(t, err, "Search of a.fx")
			assert.Equal(t, got.Items, symbol("search"), "the matches of the answer")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of the resolve port", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic})
			got, err := query.New(c, routes).Resolve(t.Context(), engine.Request{Scope: "a.fx"}, source.Position{})
			assert.NoError(t, err, "Resolve in a.fx")
			assert.Equal(t, got.Items, symbol("resolve"), "the declarations of the answer")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the relations of the relate port", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic})
			got, err := query.New(c, routes).Relate(t.Context(), engine.Request{Scope: "a.fx"}, "", sema.References)
			assert.NoError(t, err, "Relate in a.fx")
			assert.Equal(t, got.Items, []sema.Relation{{Via: "relate"}}, "the relations of the answer")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the findings of the verify port", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t, answering{name: "parser", fidelity: trust.Syntactic})
			got, err := query.New(c, routes).Verify(t.Context(), engine.Request{Scope: "a.fx"}, nil)
			assert.NoError(t, err, "Verify of a.fx")
			assert.Equal(t, got.Items, []edit.Finding{{Diagnostic: diag.Diagnostic{Message: "verify"}}},
				"the findings of the answer")
		})
	})
}
