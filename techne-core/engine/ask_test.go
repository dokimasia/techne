// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestLanguages(t *testing.T) {
	t.Parallel()

	t.Run("a request naming a language", func(t *testing.T) {
		t.Parallel()

		t.Run("is believed over the path", func(t *testing.T) {
			t.Parallel()
			// A caller may know about a path the router does not: a
			// generated file, an extension nobody claimed yet.
			got := engine.Languages(claiming{}, engine.Request{Scope: "a.fx", Language: "other"})
			assert.Equal(t, got, []source.Language{"other"},
				"a caller that says what it is asking about is answered about that")
		})
	})

	t.Run("a path an extension claims", func(t *testing.T) {
		t.Parallel()

		t.Run("goes to the language claiming it", func(t *testing.T) {
			t.Parallel()
			got := engine.Languages(claiming{}, engine.Request{Scope: "src/a.fx"})
			assert.Equal(t, got, []source.Language{fixture},
				"a file belongs to whichever language claims its extension")
		})

		t.Run("goes nowhere when nothing claims it", func(t *testing.T) {
			t.Parallel()
			// Asking every language about every unclaimed file is the
			// alternative, and it costs a parse per language per file.
			assert.Empty(t, engine.Languages(claiming{}, engine.Request{Scope: "README.md"}),
				"a file no language claims is a capability gap, not a question for all of them")
		})
	})

	t.Run("a path carrying no extension", func(t *testing.T) {
		t.Parallel()

		t.Run("goes to every language", func(t *testing.T) {
			t.Parallel()
			// A directory holds whatever it holds: a package in Go,
			// mixed sources anywhere else.
			got := engine.Languages(claiming{}, engine.Request{Scope: "src"})
			assert.Equal(t, got, []source.Language{fixture, other},
				"a directory is asked of everything, and the router's order keeps the answer steady")
		})
	})

	t.Run("the workspace root", func(t *testing.T) {
		t.Parallel()

		t.Run("goes to every language, though it looks like a file", func(t *testing.T) {
			t.Parallel()
			// path.Ext(".") is ".", so the root reads as a file carrying
			// an extension nobody claims. It is the scope a request
			// naming none is given, so getting this wrong leaves every
			// such request answered by nothing.
			got := engine.Languages(claiming{}, engine.Request{Scope: engine.Root})
			assert.NotEmpty(t, got, "a request that names no scope asks about the whole workspace")
		})
	})
}

func TestAsk(t *testing.T) {
	t.Parallel()

	t.Run("Ask", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the first engine that answers", func(t *testing.T) {
			t.Parallel()
			got, ok, _, err := engine.Ask(t.Context(), holding(t, answering{name: "first"}, answering{name: "second"}),
				fixture, engine.RoleOutline, trust.None, outlining)

			assert.NoError(t, err, "asking an engine that answers succeeds")
			assert.True(t, ok, "something answered")
			assert.Equal(t, got.Provenance.Engine, "first",
				"the catalogue orders by evidence, so the first is the strongest")
		})

		t.Run("carries out why every engine declined", func(t *testing.T) {
			t.Parallel()
			// What an engine said is often the only actionable thing in
			// the exchange: a server naming the toolchain it cannot find
			// tells a caller what to install, where "no engine serves
			// this file" tells it the language is unsupported.
			_, ok, declined, err := engine.Ask(t.Context(),
				holding(t, answering{name: "first", declines: true},
					answering{name: "second", declines: true}),
				fixture, engine.RoleOutline, trust.Syntactic, outlining)

			assert.NoError(t, err, "a decline is not a failure")
			assert.False(t, ok, "and nothing answered")
			assert.Length(t, declined, 2, "one reason per engine that could have")
			assert.Contains(t, declined.Reason(), "not this one",
				"carrying what the engine actually said")
			assert.Contains(t, declined.Reason(), "first", "and which engine said it")
		})

		t.Run("names the engine only where the reason does not", func(t *testing.T) {
			t.Parallel()
			// An engine's own error carries its name by convention. A
			// reason opening with the name twice reads as two engines
			// refusing rather than one.
			_, _, declined, err := engine.Ask(t.Context(),
				holding(t, answering{name: "named", declines: true, owns: true}),
				fixture, engine.RoleOutline, trust.Syntactic, outlining)

			assert.NoError(t, err, "a decline is not a failure")
			assert.Equal(t, strings.Count(declined.Reason(), "named"), 1,
				"attributed once, however the engine wrote its own error")
		})

		t.Run("carries nothing out when one answered", func(t *testing.T) {
			t.Parallel()
			_, ok, declined, err := engine.Ask(t.Context(),
				holding(t, answering{name: "first"}),
				fixture, engine.RoleOutline, trust.Syntactic, outlining)

			assert.NoError(t, err, "asking succeeds")
			assert.True(t, ok, "and an engine answered")
			assert.Empty(t, declined, "so there is nothing to explain")
		})

		t.Run("moves on from an engine that declines", func(t *testing.T) {
			t.Parallel()
			// Declining serves the role and cannot answer this request,
			// which is different from being broken.
			got, ok, _, err := engine.Ask(t.Context(),
				holding(t, answering{name: "first", declines: true}, answering{name: "second"}),
				fixture, engine.RoleOutline, trust.None, outlining)

			assert.NoError(t, err, "an engine declining one request is not a fault")
			assert.True(t, ok, "the next engine got a turn")
			assert.Equal(t, got.Provenance.Engine, "second", "and answered")
		})

		t.Run("stops at an engine that is broken", func(t *testing.T) {
			t.Parallel()
			// Answering from a weaker engine when the stronger one is
			// broken hides the breakage for as long as anyone believes
			// the answer.
			_, _, _, err := engine.Ask(t.Context(),
				holding(t, answering{name: "first", breaks: true}, answering{name: "second"}),
				fixture, engine.RoleOutline, trust.None, outlining)
			assert.HasError(t, err, "a broken engine reaches the caller rather than being routed around")
		})

		t.Run("reports that nothing answered", func(t *testing.T) {
			t.Parallel()
			_, ok, _, err := engine.Ask(t.Context(), holding(t), fixture,
				engine.RoleOutline, trust.None, outlining)
			assert.NoError(t, err, "having nothing to ask is not a fault")
			assert.False(t, ok, "and is told apart from an engine that answered with nothing")
		})

		t.Run("stamps the evidence rather than taking the engine's word", func(t *testing.T) {
			t.Parallel()
			got, _, _, err := engine.Ask(t.Context(), holding(t, answering{name: "first"}),
				fixture, engine.RoleOutline, trust.Resolved, outlining)

			assert.NoError(t, err, "asking succeeds")
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic,
				"the tier is read from the engine, which cannot state it in a result")
			assert.Equal(t, got.Status, trust.Degraded,
				"the caller asked for resolved evidence and did not get it")
		})
	})

	t.Run("AskEach", func(t *testing.T) {
		t.Parallel()

		t.Run("asks every language a directory could hold", func(t *testing.T) {
			t.Parallel()
			got, _, err := engine.AskEach(t.Context(),
				holding(t, answering{name: "fx"}, answering{name: "other", language: other}),
				claiming{}, engine.Request{Scope: "src"}, engine.RoleOutline, outlining)

			assert.NoError(t, err, "asking a directory succeeds")
			assert.Length(t, got, 2, "a directory holding two languages is answered by both")
		})
	})

	t.Run("AskAny", func(t *testing.T) {
		t.Parallel()

		t.Run("stops at the first language that answers", func(t *testing.T) {
			t.Parallel()
			// A symbol is declared in one language, so a second answer
			// would be about a second symbol and the work is wasted.
			second := &counting{answering{name: "other", language: other}, 0}
			got, ok, _, err := engine.AskAny(t.Context(),
				holding(t, answering{name: "fx"}, second),
				claiming{}, engine.Request{Scope: "src"}, engine.RoleOutline, outlining)

			assert.NoError(t, err, "asking succeeds")
			assert.True(t, ok, "the first language answered")
			assert.Equal(t, got.Provenance.Engine, "fx", "and its answer is the one returned")
			assert.Equal(t, second.asked, 0, "the second language was never asked")
		})
	})

	t.Run("Unsupported", func(t *testing.T) {
		t.Parallel()

		t.Run("carries no payload and says why", func(t *testing.T) {
			t.Parallel()
			got := engine.Unsupported[sema.Symbol]("nothing serves fixture")
			assert.Empty(t, got.Items, "an empty list is never read as evidence of absence")
			assert.False(t, got.Provenance.SupportsNegativeClaim(),
				"an answer nothing produced proves nothing about what is there")
			assert.Equal(t, got.Provenance.Caveats[0].Note, "nothing serves fixture",
				"a caller told only no cannot tell a gap from a mistake it could correct")
		})
	})
}

const (
	fixture = source.Language("fixture")
	other   = source.Language("other")
)

// outlining is the one line that differs between roles.
func outlining(e engine.Engine) (engine.Result[sema.Symbol], error) {
	return e.(engine.Outliner).Outline(context.Background(), engine.Request{})
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

// claiming routes .fx to one language and knows two.
type claiming struct{}

func (claiming) LanguageOf(p source.Path) (source.Language, bool) {
	if len(p) > 3 && p[len(p)-3:] == ".fx" {
		return fixture, true
	}
	return "", false
}

func (claiming) Languages() []source.Language { return []source.Language{fixture, other} }

// answering is an engine that outlines, declines or breaks.
type answering struct {
	name     string
	language source.Language
	declines bool
	// owns writes the engine's own name into its decline, which is what
	// the repository's error convention produces.
	owns   bool
	breaks bool
}

func (a answering) Name() string { return a.name }

func (a answering) Language() source.Language {
	if a.language == "" {
		return fixture
	}
	return a.language
}

func (answering) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (answering) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (a answering) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	switch {
	case a.declines && a.owns:
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: %s: not this one",
			engine.ErrDecline, a.name)
	case a.declines:
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: not this one", engine.ErrDecline)
	case a.breaks:
		return engine.Result[sema.Symbol]{}, fmt.Errorf("this engine is broken")
	}
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
}

// counting records whether it was asked, so work nobody wanted shows up.
type counting struct {
	answering
	asked int
}

func (c *counting) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	c.asked++
	return c.answering.Outline(ctx, req)
}
