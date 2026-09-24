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

// The services that the tools take, one interface per question. A tool takes the interfaces
// that it calls and no other, and the package that implements them does not export them.
//
// The read interfaces return an [engine.Answer], which a service publishes with the evidence
// of the engine that served it. The ports of core/engine have the same names and return an
// [engine.Result].
type (
	// Outliner returns the declarations of a scope.
	Outliner interface {
		Outline(ctx context.Context, req engine.Request) (engine.Answer[sema.Symbol], error)
	}

	// Searcher returns the declarations of a scope that match a query.
	Searcher interface {
		Search(ctx context.Context, req engine.Request, q engine.Query) (engine.Answer[sema.Symbol], error)
	}

	// Resolver returns the declarations that the name at a position denotes.
	Resolver interface {
		Resolve(ctx context.Context, req engine.Request, at source.Position) (engine.Answer[sema.Symbol], error)
	}

	// Relator returns the relations of one kind from the declaration with the ID of.
	Relator interface {
		Relate(
			ctx context.Context,
			req engine.Request,
			of sema.ID,
			kind sema.RelationKind,
		) (engine.Answer[sema.Relation], error)
	}

	// Verifier returns the findings of the checks of a language over a scope.
	Verifier interface {
		Verify(ctx context.Context, req engine.Request, suites []string) (engine.Answer[edit.Finding], error)
	}

	// Catalogue returns the roles that each engine serves.
	Catalogue interface {
		Capabilities(ctx context.Context) []engine.Capability
	}

	// Writer plans, checks and writes an operation, or plans and checks it for a dry run.
	Writer interface {
		Apply(ctx context.Context, req edit.Request) (edit.Outcome, error)
	}

	// Committer writes the change of a preview.
	Committer interface {
		Commit(ctx context.Context, handle string) (edit.Outcome, error)
	}
)
