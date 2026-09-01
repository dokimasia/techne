// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Outliner reports what a scope declares.
type Outliner interface {
	Outline(ctx context.Context, req Request) (Result[sema.Symbol], error)
}

// Searcher reports which declarations match a query.
type Searcher interface {
	Search(ctx context.Context, req Request, q Query) (Result[sema.Symbol], error)
}

// Resolver reports what the name at a position denotes. More than one
// item means the name is ambiguous, and the caller chooses.
type Resolver interface {
	Resolve(ctx context.Context, req Request, at source.Position) (Result[sema.Symbol], error)
}

// Relator reports how a symbol connects to the rest, in one direction.
// A service inverts the kind when an engine stores the other direction.
type Relator interface {
	Relate(ctx context.Context, req Request, of sema.ID, kind sema.RelationKind) (Result[sema.Relation], error)
}

// Planner computes the edits an operation would need, and writes
// nothing. It is handed no filesystem, so atomicity is implemented once
// in the write path and no adapter can weaken it.
type Planner interface {
	Plan(
		ctx context.Context,
		req Request,
		op edit.Operation,
		target edit.Target,
		args edit.Args,
	) (Result[edit.Change], error)
}

// Formatter normalises the named paths and returns the edits that would
// do it, touching nothing outside them.
type Formatter interface {
	Format(ctx context.Context, paths []source.Path) (Result[edit.Change], error)
}

// Verifier reports what a compiler or linter says about a scope, in this
// process rather than through a subprocess.
type Verifier interface {
	Verify(ctx context.Context, req Request) (Result[diag.Diagnostic], error)
}

// Indexer produces the facts an index stores, and says how far a change
// to one file reaches.
//
// Granularity is static and coarse: how far this engine's facts could
// ever reach, which an index consults once to decide whether adopting
// the engine is affordable. Affected is dynamic and precise: given that
// this file changed, exactly which paths must be indexed again.
type Indexer interface {
	Index(ctx context.Context, p source.Path) (Result[sema.Symbol], error)
	Granularity() Invalidation
	Affected(changed source.Path) []source.Path
}

// Invalidation is how far a change to one file reaches.
type Invalidation uint8

const (
	// InvalidateFile means only the changed file's facts are stale. An
	// index can adopt such an engine and reindex one file per edit.
	InvalidateFile Invalidation = iota
	// InvalidateUnit means the changed file's unit is stale.
	InvalidateUnit
	// InvalidateWorkspace means any file may be stale. An index cannot
	// afford such an engine: it would rebuild everything per keystroke
	// to serve answers a live query produces for the same price.
	InvalidateWorkspace
)
