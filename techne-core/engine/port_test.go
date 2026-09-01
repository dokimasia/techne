// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// outlineOnly serves one role. It exists to prove that an engine
// declines a capability by not having the method, rather than by
// returning an error at run time.
type outlineOnly struct{}

func (outlineOnly) Name() string                        { return "outline-only" }
func (outlineOnly) Language() source.Language           { return source.Language("go") }
func (outlineOnly) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (outlineOnly) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (outlineOnly) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
}

func TestPort(t *testing.T) {
	t.Parallel()

	t.Run("satisfaction", func(t *testing.T) {
		t.Parallel()

		t.Run("one role is enough to be an engine", func(t *testing.T) {
			t.Parallel()
			var e engine.Engine = outlineOnly{}
			_, serves := e.(engine.Outliner)
			assert.True(t, serves, "one role is enough: an engine need not implement ports it cannot serve")
		})

		t.Run("a role not implemented is not claimed", func(t *testing.T) {
			t.Parallel()
			// The catalogue selects by type assertion. An engine that
			// claimed every role and errored at run time would advertise
			// capabilities that are not there.
			var e engine.Engine = outlineOnly{}
			for name, claimed := range map[string]bool{
				"Searcher":  assertSearcher(e),
				"Resolver":  assertResolver(e),
				"Relator":   assertRelator(e),
				"Planner":   assertPlanner(e),
				"Formatter": assertFormatter(e),
				"Verifier":  assertVerifier(e),
				"Indexer":   assertIndexer(e),
			} {
				_ = name
				assert.False(t, claimed,
					"selection is by type assertion, so an engine declines a role by lacking the method")
			}
		})
	})
}

func assertSearcher(e engine.Engine) bool  { _, ok := e.(engine.Searcher); return ok }
func assertResolver(e engine.Engine) bool  { _, ok := e.(engine.Resolver); return ok }
func assertRelator(e engine.Engine) bool   { _, ok := e.(engine.Relator); return ok }
func assertPlanner(e engine.Engine) bool   { _, ok := e.(engine.Planner); return ok }
func assertFormatter(e engine.Engine) bool { _, ok := e.(engine.Formatter); return ok }
func assertVerifier(e engine.Engine) bool  { _, ok := e.(engine.Verifier); return ok }
func assertIndexer(e engine.Engine) bool   { _, ok := e.(engine.Indexer); return ok }
