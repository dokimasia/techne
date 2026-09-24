// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"cmp"
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// The languages of the test engines.
const (
	fixture = source.Language("fixture")
	other   = source.Language("other")
)

// fake is an engine that implements Outliner and no other port. It declares
// one fidelity and one cost for every role, and serves fixture unless
// language is set. Outline returns err when it is set, and counts its calls
// in calls when calls is set.
type fake struct {
	name     string
	language source.Language
	fidelity trust.Fidelity
	cost     engine.Cost
	err      error
	calls    *int
	// skipped makes Outline return a skipped result.
	skipped bool
}

func (f fake) Name() string                        { return f.name }
func (f fake) Language() source.Language           { return cmp.Or(f.language, fixture) }
func (f fake) Fidelity(engine.Role) trust.Fidelity { return f.fidelity }
func (f fake) Cost(engine.Role) engine.Cost        { return f.cost }

func (f fake) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	if f.calls != nil {
		*f.calls++
	}
	if f.err != nil {
		return engine.Result[sema.Symbol]{}, f.err
	}
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal, Skipped: f.skipped}, nil
}

// complete is a fake that implements every port and Available. Available
// returns unusable, and counts its calls in checks when checks is set.
type complete struct {
	fake
	unusable error
	checks   *int
}

func (c complete) Available(context.Context) error {
	if c.checks != nil {
		*c.checks++
	}
	return c.unusable
}

func (complete) Search(context.Context, engine.Request, engine.Query) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}

func (complete) Resolve(context.Context, engine.Request, source.Position) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}

func (complete) Relate(
	context.Context,
	engine.Request,
	sema.ID,
	sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	return engine.Result[sema.Relation]{}, nil
}

func (complete) Plan(
	context.Context,
	engine.Request,
	edit.Operation,
	edit.Target,
	edit.Args,
) (engine.Result[edit.Change], error) {
	return engine.Result[edit.Change]{}, nil
}

func (complete) Format(context.Context, []source.Path) (engine.Result[edit.Change], error) {
	return engine.Result[edit.Change]{}, nil
}

func (complete) Check(context.Context, map[source.Path][]byte) (engine.Result[edit.Finding], error) {
	return engine.Result[edit.Finding]{}, nil
}

func (complete) Verify(context.Context, engine.Request, []string) (engine.Result[edit.Finding], error) {
	return engine.Result[edit.Finding]{}, nil
}

func (complete) Index(context.Context, source.Path) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}

func (complete) Granularity() engine.Invalidation   { return engine.InvalidateFile }
func (complete) Affected(source.Path) []source.Path { return nil }

var (
	_ engine.Outliner  = complete{}
	_ engine.Searcher  = complete{}
	_ engine.Resolver  = complete{}
	_ engine.Relator   = complete{}
	_ engine.Planner   = complete{}
	_ engine.Formatter = complete{}
	_ engine.Checker   = complete{}
	_ engine.Verifier  = complete{}
	_ engine.Indexer   = complete{}
	_ engine.Available = complete{}
)

func TestPort(t *testing.T) {
	t.Parallel()

	t.Run("Invalidation", func(t *testing.T) {
		t.Parallel()

		t.Run("orders granularities from narrowest to widest", func(t *testing.T) {
			t.Parallel()
			widening := []engine.Invalidation{
				engine.InvalidateFile, engine.InvalidateUnit, engine.InvalidateWorkspace,
			}
			assert.Pairwise(t, widening, func(narrow, wide engine.Invalidation) bool {
				return narrow < wide
			}, "granularities")
		})

		t.Run("is InvalidateFile when zero", func(t *testing.T) {
			t.Parallel()
			var zero engine.Invalidation
			assert.Equal(t, zero, engine.InvalidateFile, "zero value")
		})
	})
}
