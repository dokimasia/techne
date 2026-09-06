// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
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

// tabbing is the indentation a server is told to assume where a file
// gives it nothing to go on.
//
// The protocol requires both fields and every server that formats has
// its own answer already — gofmt uses tabs whatever it is told, and a
// project with an editorconfig has the server read that. What is sent
// matters only for a server with no other source, and four spaces is
// what most of those default to.
const (
	tabbing = 4
	spaces  = true
)

// Format normalises the named paths and returns the edits that would do
// it, touching nothing.
//
// The language's own formatter, reached through its server: gofmt behind
// gopls, the TypeScript formatter behind typescript-language-server. A
// caller gets what the language's own tooling would produce rather than
// what techne thinks the language should look like.
//
// Nothing here writes. The edits go back through techne's write path,
// which reads the files, gates the result and applies it atomically.
func (e *Engine) Format(ctx context.Context, paths []source.Path) (engine.Result[edit.Change], error) {
	// None of them this engine's is a set to leave alone, and settled
	// before a server is started rather than after: a caller naming a
	// mixed set otherwise starts one server per language in it.
	if !slices.ContainsFunc(paths, func(p source.Path) bool {
		return lang.Claims(string(p), e.declared.Extensions)
	}) {
		return engine.Result[edit.Change]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !provides(held.capable.DocumentFormattingProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/formatting")
	}

	var out []edit.Change
	var large []source.Path
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return engine.Result[edit.Change]{}, err
		}
		// A path of another language is not this engine's to touch. A
		// caller naming a mixed set gets each file from whoever claims
		// it rather than a refusal for the whole set.
		if !lang.Claims(string(p), e.declared.Extensions) {
			continue
		}
		if opened := e.open(ctx, held, p); opened != nil {
			// A file past the size an engine reads is left as it is
			// rather than costing the rest of the set its formatting.
			if _, big := errors.AsType[lang.LargeError](opened); big {
				large = append(large, p)
				continue
			}
			return engine.Result[edit.Change]{}, opened
		}

		edits, err := held.asks.Formatting(ctx, &protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
			Options: protocol.FormattingOptions{
				TabSize: tabbing, InsertSpaces: spaces,
			},
		})
		if err != nil {
			return engine.Result[edit.Change]{}, fmt.Errorf("lsp: %s: format %s: %w",
				e.server.Name, p, err)
		}
		if len(edits) == 0 {
			// Already as the formatter would write it. A change with no
			// edits would have the write path rewrite a file to itself.
			continue
		}

		change, err := e.rewriteAgainst(uri.File(e.fullPath(p)), edits, nil)
		if err != nil {
			return engine.Result[edit.Change]{}, err
		}
		out = append(out, change)
	}

	covered := trust.ScopeTotal
	if len(large) > 0 {
		covered = trust.ScopePartial
	}
	return engine.Result[edit.Change]{
		Items:        out,
		Completeness: covered,
		Caveats:      append([]trust.Caveat{dynamic}, unread(large)...),
	}, nil
}
