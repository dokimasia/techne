// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"slices"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// The formatting options of textDocument/formatting. LSP 3.17 requires both. gopls formats
// with tabs whatever they say, and a server that reads .editorconfig prefers it.
const (
	tabSize      = 4
	insertSpaces = true
)

// Format returns one change per path that the formatter of the language rewrites, from
// textDocument/formatting, and writes nothing.
//
// A path of another language is left out, and a set without a path of the language returns a
// skipped result without starting the server. A file that [lang.Readable] refuses is left out
// and named in a [trust.CaveatUnread] caveat, and the result is partial. A file already
// formatted has no change. A server that does not answer within [Server.Answering] returns
// [engine.ErrDecline].
func (e *Engine) Format(ctx context.Context, paths []source.Path) (engine.Result[edit.Change], error) {
	out, err := e.formatting(ctx, paths)
	return out, e.unanswered(ctx, err)
}

// formatting is [Engine.Format] before a missed deadline becomes a decline.
func (e *Engine) formatting(ctx context.Context, paths []source.Path) (engine.Result[edit.Change], error) {
	mine := slices.DeleteFunc(slices.Clone(paths), func(p source.Path) bool {
		return !lang.Claims(string(p), e.declared.Extensions)
	})
	if len(mine) == 0 {
		return engine.Result[edit.Change]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()
	if !provides(held.capable.DocumentFormattingProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/formatting")
	}

	var out []edit.Change
	var skipped []source.Path
	for _, p := range mine {
		if err := ctx.Err(); err != nil {
			return engine.Result[edit.Change]{}, err
		}
		doc, err := e.open(ctx, held, p)
		if refused(err) {
			skipped = append(skipped, p)
			continue
		}
		if err != nil {
			return engine.Result[edit.Change]{}, err
		}
		edits, err := held.asks.Formatting(ctx, &protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
			Options:      protocol.FormattingOptions{TabSize: tabSize, InsertSpaces: insertSpaces},
		})
		if err != nil {
			return engine.Result[edit.Change]{}, fmt.Errorf("lsp: %s: format %s: %w", e.server.Name, p, err)
		}
		if len(edits) == 0 {
			continue
		}
		converted, err := e.textEdits(doc, edits)
		if err != nil {
			return engine.Result[edit.Change]{}, err
		}
		out = append(out, edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: converted})
	}

	covered := trust.ScopeTotal
	if len(skipped) > 0 {
		covered = trust.ScopePartial
	}
	return engine.Result[edit.Change]{
		Items:        out,
		Completeness: covered,
		Caveats:      append([]trust.Caveat{dynamic}, unread(skipped)...),
	}, nil
}
