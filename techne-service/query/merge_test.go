// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/query"
)

const other = source.Language("other")

// tongue is an engine for whichever language a case gives it.
type tongue struct {
	engine   outliner
	language source.Language
	coverage trust.Completeness
	caveats  []trust.Caveat
}

func (g tongue) Name() string                          { return g.engine.name }
func (g tongue) Language() source.Language             { return g.language }
func (g tongue) Fidelity(r engine.Role) trust.Fidelity { return g.engine.Fidelity(r) }
func (g tongue) Cost(r engine.Role) engine.Cost        { return g.engine.Cost(r) }

func (g tongue) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{
		Items:        g.engine.found,
		Completeness: g.coverage,
		Caveats:      g.caveats,
	}, nil
}

// carrying returns the engine with a caveat attached to its answer.
func carrying(g tongue, c ...trust.Caveat) tongue {
	g.caveats = c
	return g
}

// speaking builds an engine for one language at one tier.
func speaking(name string, l source.Language, f trust.Fidelity, c trust.Completeness, found string) tongue {
	return tongue{
		engine:   outliner{name: name, fidelity: f, found: symbol(found)},
		language: l,
		coverage: c,
	}
}

// tongues routes a file suffix per language and knows both.
type tongues struct{}

func (tongues) LanguageOf(p source.Path) (source.Language, bool) {
	switch {
	case len(p) > 3 && p[len(p)-3:] == ".fx":
		return fixture, true
	case len(p) > 3 && p[len(p)-3:] == ".ot":
		return other, true
	default:
		return "", false
	}
}

func (tongues) Languages() []source.Language { return []source.Language{fixture, other} }

func both(t *testing.T, engines ...engine.Engine) *query.Service {
	t.Helper()
	return query.New(catalogue(t, engines...), tongues{})
}

func TestMerge(t *testing.T) {
	t.Parallel()

	t.Run("a directory scope", func(t *testing.T) {
		t.Parallel()

		t.Run("asks every language, because a directory holds several", func(t *testing.T) {
			t.Parallel()
			// A directory is a package in Go and a folder of mixed
			// sources anywhere else. Refusing it because it carries no
			// extension would make the commonest scope unusable.
			s := both(t,
				speaking("fx", fixture, trust.Resolved, trust.ScopeTotal, "FromFixture"),
				speaking("ot", other, trust.Resolved, trust.ScopeTotal, "FromOther"),
			)
			got, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Equal(t, got.Status, trust.OK, "every language answered")
			assert.Length(t, got.Items, 2, "the answer holds what both languages declared")
		})

		t.Run("names every engine that answered", func(t *testing.T) {
			t.Parallel()
			s := both(t,
				speaking("fx", fixture, trust.Resolved, trust.ScopeTotal, "A"),
				speaking("ot", other, trust.Resolved, trust.ScopeTotal, "B"),
			)
			got, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Contains(t, got.Provenance.Engine, "fx", "a merged answer names each engine behind it")
			assert.Contains(t, got.Provenance.Engine, "ot", "a merged answer names each engine behind it")
		})

		t.Run("states a shared caveat once, however many languages carried it", func(t *testing.T) {
			t.Parallel()
			// Every parser says the same thing about a name it matched.
			// Five copies of that sentence spend a caller's context on
			// one fact, which is what the budget exists to prevent.
			shared := trust.Caveat{
				Code: trust.CaveatDynamic,
				Note: "a parser matched text: a name resolved across files is coincidence",
			}
			got, err := both(t,
				carrying(speaking("fx", fixture, trust.Syntactic, trust.ScopeTotal, "A"), shared),
				carrying(speaking("ot", other, trust.Syntactic, trust.ScopeTotal, "B"), shared),
			).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Length(t, got.Provenance.Caveats, 1,
				"one fact about the answer is stated once")
		})

		t.Run("keeps two caveats that name different files", func(t *testing.T) {
			t.Parallel()
			// The paths are what the caveat is about, so two of them are
			// two facts rather than one repeated.
			code := trust.CaveatDynamic
			got, err := both(t,
				carrying(speaking("fx", fixture, trust.Syntactic, trust.ScopeTotal, "A"),
					trust.Caveat{Code: code, Note: "unparsed", Paths: []source.Path{"a.fx"}}),
				carrying(speaking("ot", other, trust.Syntactic, trust.ScopeTotal, "B"),
					trust.Caveat{Code: code, Note: "unparsed", Paths: []source.Path{"b.ot"}}),
			).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Length(t, got.Provenance.Caveats, 2,
				"a caveat naming a file is about that file, and dropping one loses it")
		})

		t.Run("claims only the weakest evidence behind it", func(t *testing.T) {
			t.Parallel()
			// Half the answer came from a parser. Claiming resolved
			// would let a caller trust the whole of it.
			s := both(t,
				speaking("fx", fixture, trust.Resolved, trust.ScopeTotal, "A"),
				speaking("ot", other, trust.Syntactic, trust.ScopeTotal, "B"),
			)
			got, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic,
				"an answer is only as strong as its weakest part")
			assert.False(t, got.Provenance.SupportsNegativeClaim(),
				"one parser among the contributors means the whole proves nothing")
		})

		t.Run("claims only the weakest coverage behind it", func(t *testing.T) {
			t.Parallel()
			s := both(t,
				speaking("fx", fixture, trust.Resolved, trust.ScopeTotal, "A"),
				speaking("ot", other, trust.Resolved, trust.ScopePartial, "B"),
			)
			got, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Equal(t, got.Provenance.Completeness, trust.ScopePartial,
				"one language that missed files makes the whole answer partial")
		})

		t.Run("answers even when only one language has files there", func(t *testing.T) {
			t.Parallel()
			// A language serving nothing in this directory is not a
			// capability gap.
			s := both(t,
				speaking("fx", fixture, trust.Syntactic, trust.ScopeTotal, "A"),
			)
			got, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Equal(t, got.Status, trust.OK, "the language that serves this tree answered")
			assert.Length(t, got.Items, 1, "what it found comes back")
		})

		t.Run("is unsupported only when nothing serves any language", func(t *testing.T) {
			t.Parallel()
			got, err := query.New(engine.NewCatalog(), tongues{}).
				Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "having nothing to ask is not a fault")
			assert.Equal(t, got.Status, trust.Unsupported, "no engine serves any registered language")
		})

		t.Run("answers two identical requests identically", func(t *testing.T) {
			t.Parallel()
			s := both(t,
				speaking("fx", fixture, trust.Resolved, trust.ScopeTotal, "A"),
				speaking("ot", other, trust.Resolved, trust.ScopeTotal, "B"),
			)
			first, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			second, err := s.Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "a directory is a scope, not a fault")
			assert.Equal(t, second.Items, first.Items,
				"the languages are asked in a fixed order, so a merged answer does not wander")
		})
	})

	t.Run("a file scope", func(t *testing.T) {
		t.Parallel()

		t.Run("still asks only the language claiming it", func(t *testing.T) {
			t.Parallel()
			s := both(t,
				speaking("fx", fixture, trust.Syntactic, trust.ScopeTotal, "A"),
				speaking("ot", other, trust.Syntactic, trust.ScopeTotal, "B"),
			)
			got, err := s.Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "a file that routes is answered")
			assert.Length(t, got.Items, 1, "a file belongs to one language")
			assert.Equal(t, got.Provenance.Engine, "fx", "the language claiming the suffix answered")
		})
	})

	t.Run("an engine that read nothing", func(t *testing.T) {
		t.Parallel()

		t.Run("does not lower what the others are worth", func(t *testing.T) {
			t.Parallel()
			// A directory with no Ruby in it tells you nothing about
			// Ruby. Letting a parser that opened no file drag a type
			// checker down reports resolved evidence as text-matched and
			// withdraws a negative claim the caller had earned.
			got, err := query.New(holding(t,
				strong{}, absent{},
			), everything{}).Outline(t.Context(), engine.Request{Scope: "src"})

			assert.NoError(t, err, "outlining a directory succeeds")
			assert.Equal(t, got.Provenance.Fidelity, trust.Resolved,
				"the engine that read the files decides what the answer is worth")
			assert.True(t, got.Provenance.SupportsNegativeClaim(),
				"and the claim it earned survives being asked beside a language with no files")
			assert.NotContains(t, got.Provenance.Engine, "absent",
				"an engine with nothing to say is not named as having said it")
		})

		t.Run("is told apart from one that read and found nothing", func(t *testing.T) {
			t.Parallel()
			// An engine that searched forty files and matched none still
			// cannot say there are no others, and its silence is what
			// stops the merged answer claiming there are.
			got, err := query.New(holding(t,
				strong{}, quiet{},
			), everything{}).Outline(t.Context(), engine.Request{Scope: "src"})

			assert.NoError(t, err, "outlining a directory succeeds")
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic,
				"a parser that read files is evidence, however little it found")
			assert.False(t, got.Provenance.SupportsNegativeClaim(),
				"so nothing here may claim there are no others")
		})

		t.Run("counts when every engine read nothing", func(t *testing.T) {
			t.Parallel()
			// A scope holding no source at all is one nothing examined.
			// Reporting the strongest tier among engines that read
			// nothing would claim evidence none of them gathered.
			got, err := query.New(holding(t,
				absent{}, quietAbsent{},
			), everything{}).Outline(t.Context(), engine.Request{Scope: "src"})

			assert.NoError(t, err, "outlining an empty directory succeeds")
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic,
				"nothing was read, so the weakest engine that looked decides")
		})
	})
}

// everything routes a directory to every language it knows.
type everything struct{}

func (everything) LanguageOf(source.Path) (source.Language, bool) { return "", false }

func (everything) Languages() []source.Language {
	return []source.Language{"strong", "quiet", "absent", "quiet-absent"}
}

// holding builds a catalogue over the engines given.
func holding(t *testing.T, engines ...engine.Engine) *engine.Catalog {
	t.Helper()
	c := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, c.Add(e), "a test engine registers")
	}
	return c
}

// strong read the files and bound the names in them.
type strong struct{}

func (strong) Name() string                        { return "strong" }
func (strong) Language() source.Language           { return "strong" }
func (strong) Fidelity(engine.Role) trust.Fidelity { return trust.Resolved }
func (strong) Cost(engine.Role) engine.Cost        { return engine.CostAnalyze }

func (strong) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{
		Items:        []sema.Symbol{{Name: "Store", Kind: sema.KindType, Language: "strong"}},
		Completeness: trust.ScopeTotal,
	}, nil
}

// quiet read the files and found nothing in them.
type quiet struct{}

func (quiet) Name() string                        { return "quiet" }
func (quiet) Language() source.Language           { return "quiet" }
func (quiet) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (quiet) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (quiet) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
}

// absent found no file of its own to read.
type absent struct{ quiet }

func (absent) Name() string              { return "absent" }
func (absent) Language() source.Language { return "absent" }

func (absent) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal, Skipped: true}, nil
}

// quietAbsent is a second one, so a scope nothing read has two engines
// saying so.
type quietAbsent struct{ absent }

func (quietAbsent) Name() string              { return "quiet-absent" }
func (quietAbsent) Language() source.Language { return "quiet-absent" }
