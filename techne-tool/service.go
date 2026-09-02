// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// The services a tool is given, one interface per question asked.
//
// A tool takes only what it uses, so a change to the read service that
// no tool calls cannot break a tool's tests. They are declared here, in
// the package that consumes them, rather than exported by the packages
// that implement them: what a consumer needs is the consumer's to state.
//
// The names mirror core/engine's ports because they are the same
// questions one level up. What tells them apart is the answer: an engine
// returns a [engine.Result], which carries what it found and nothing it
// could use to overstate itself, and a service returns an
// [engine.Answer], which carries the evidence a service stamped on it.
type (
	// Outliner reports what a scope declares.
	Outliner interface {
		Outline(ctx context.Context, req engine.Request) (engine.Answer[sema.Symbol], error)
	}

	// Searcher reports which declarations match a query.
	Searcher interface {
		Search(ctx context.Context, req engine.Request, q engine.Query) (engine.Answer[sema.Symbol], error)
	}

	// Resolver reports what the name at a position denotes.
	Resolver interface {
		Resolve(ctx context.Context, req engine.Request, at source.Position) (engine.Answer[sema.Symbol], error)
	}

	// Relator reports how a declaration connects to the rest, in one
	// direction.
	Relator interface {
		Relate(
			ctx context.Context,
			req engine.Request,
			of sema.ID,
			kind sema.RelationKind,
		) (engine.Answer[sema.Relation], error)
	}

	// Verifier reports what a language's own gate says about a scope.
	Verifier interface {
		Verify(ctx context.Context, req engine.Request, suites []string) (engine.Answer[edit.Finding], error)
	}

	// Catalogue reports what the system can answer, per language and
	// role.
	Catalogue interface {
		Capabilities(ctx context.Context) []engine.Capability
	}

	// Writer applies an operation to the workspace, or reports what
	// applying it would do.
	Writer interface {
		Apply(ctx context.Context, req edit.Request) (edit.Outcome, error)
	}
)
