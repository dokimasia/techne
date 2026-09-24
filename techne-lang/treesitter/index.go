// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// Index returns the declarations of one file, as Outline returns them for
// that file. It returns a result with Skipped set for a file of another
// language, and the error of [lang.Readable] for a file that the workspace
// excludes or that is larger than [lang.Largest].
func (e *Engine) Index(ctx context.Context, p source.Path) (engine.Result[sema.Symbol], error) {
	if !lang.Claims(string(p), e.declared.Extensions) {
		return result[sema.Symbol](nil, lang.Files{}, matchedText), nil
	}
	if err := ctx.Err(); err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	content, err := e.contents(p)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	declared, _, err := e.declarations(p, content, nil)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	return result(declared, lang.Files{Read: []source.Path{p}}, matchedText), nil
}

// Granularity returns [engine.InvalidateFile]. A parser reads one file at a
// time and does not bind names across files, so a change to one file leaves
// the facts about every other file valid.
func (*Engine) Granularity() engine.Invalidation { return engine.InvalidateFile }

// Affected returns changed alone, as [Engine.Granularity] states.
func (*Engine) Affected(changed source.Path) []source.Path {
	return []source.Path{changed}
}
