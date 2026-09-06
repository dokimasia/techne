// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"io/fs"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Index reports what one file declares, for an index to store.
//
// The same facts [Engine.Outline] returns for that file. It is a
// separate port because an index asks per file and needs to know what a
// change to one costs, which an outline does not answer.
func (e *Engine) Index(ctx context.Context, p source.Path) (engine.Result[sema.Symbol], error) {
	if !lang.Claims(string(p), e.declared.Extensions) {
		// Not this language's file. Skipped rather than answered with
		// nothing, so an index storing the answer does not record that
		// the file declares none.
		return engine.Result[sema.Symbol]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}
	if err := ctx.Err(); err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	// A caller naming a file reaches this without passing a walk, so the
	// walk's rules are asked for here. A bundle the workspace calls
	// generated is not indexed just because something enumerated it.
	if unreadable := lang.Readable(e.fsys, p); unreadable != nil {
		return engine.Result[sema.Symbol]{}, unreadable
	}

	content, err := fs.ReadFile(e.fsys, string(p))
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("treesitter: read %s: %w", p, err)
	}
	declared, err := e.declarations(p, content)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	return found(declared, 1, nil), nil
}

// Granularity is [engine.InvalidateFile].
//
// A parser reads one file and resolves nothing across files, so its
// facts about one file cannot go stale because another changed. That is
// what makes a parser affordable to index: an edit costs one file's
// worth of work rather than a rebuild.
func (*Engine) Granularity() engine.Invalidation { return engine.InvalidateFile }

// Affected is the changed file and nothing else, for the reason
// [Engine.Granularity] gives.
func (*Engine) Affected(changed source.Path) []source.Path {
	return []source.Path{changed}
}
