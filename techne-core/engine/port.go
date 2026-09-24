// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Outliner lists the declarations in a scope.
type Outliner interface {
	Outline(ctx context.Context, req Request) (Result[sema.Symbol], error)
}

// Searcher finds the declarations in a scope that match a query.
type Searcher interface {
	Search(ctx context.Context, req Request, q Query) (Result[sema.Symbol], error)
}

// Resolver returns the declarations that the name at a position denotes.
// More than one item means the name is ambiguous.
//
// The position can contain only a line and a column. The engine computes
// the offset from the file, so Resolve is the one port where
// [source.Position.Offset] is not authoritative on input.
type Resolver interface {
	Resolve(ctx context.Context, req Request, at source.Position) (Result[sema.Symbol], error)
}

// Relator returns the relations of one kind from a declaration. The engine
// returns them in the direction that kind names, whichever direction it
// stores, and may stop at the [Request.Limit] of the caller.
// [Request.Declared] is the span at which the caller found the declaration.
type Relator interface {
	Relate(ctx context.Context, req Request, of sema.ID, kind sema.RelationKind) (Result[sema.Relation], error)
}

// Planner computes the changes an operation makes. It does not write them.
// The write path seals, checks and writes the changes, so atomicity does not
// depend on the engine.
type Planner interface {
	Plan(
		ctx context.Context,
		req Request,
		op edit.Operation,
		target edit.Target,
		args edit.Args,
	) (Result[edit.Change], error)
}

// Formatter returns the changes that format paths. The changes do not touch
// any other path.
type Formatter interface {
	Format(ctx context.Context, paths []source.Path) (Result[edit.Change], error)
}

// Verifier reports the findings of the language's toolchain for a scope on
// disk. Suites names the checks to run in the language's own terms, such as
// linters or test runners. An empty list runs the language's default. An
// engine with one check ignores suites.
type Verifier interface {
	Verify(ctx context.Context, req Request, suites []string) (Result[edit.Finding], error)
}

// Checker reports the findings for content that is not on disk. The write
// path calls it on the projected content of a change before it writes, and
// a dry run calls it without writing.
//
// A Checker judges content in memory. A [Verifier] judges the workspace on
// disk. An engine can implement either port without the other.
type Checker interface {
	Check(ctx context.Context, files map[source.Path][]byte) (Result[edit.Finding], error)
}

// Indexer produces the declarations an index stores for one file.
//
// Granularity returns the widest invalidation a change can cause. It is
// fixed for the engine's lifetime. Affected returns the paths to index again
// after a change to one path.
type Indexer interface {
	Index(ctx context.Context, p source.Path) (Result[sema.Symbol], error)
	Granularity() Invalidation
	Affected(changed source.Path) []source.Path
}

// Invalidation is the set of files that a change to one file makes stale.
// The values are ordered from the narrowest to the widest. The zero value is
// InvalidateFile.
type Invalidation uint8

const (
	// InvalidateFile marks only the changed file stale.
	InvalidateFile Invalidation = iota
	// InvalidateUnit marks every file of the changed file's unit stale.
	InvalidateUnit
	// InvalidateWorkspace marks every file stale, so the index rebuilds
	// everything after each change.
	InvalidateWorkspace
)
