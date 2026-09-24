// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"time"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// Verify returns the findings of the server for the files of the scope on disk. It ignores
// suites, because a server runs one analysis, and adds a [trust.CaveatUnsupported] caveat when
// req names any.
//
// Verify opens every file of the scope, waits for the server to settle, and then collects the
// diagnostics of each file: from textDocument/diagnostic for a server that offers pull
// diagnostics, and from the diagnostics the server publishes otherwise. All files share one
// wait of [reporting] for published diagnostics.
//
// The result is partial when the server had not settled, when a file received no report, or
// when the scope contains a file larger than [lang.Largest], and a caveat names the reason.
// A file that received no report is a file the server did not analyse, so its absence of
// findings is no evidence. For a scope without a file of the language the result is skipped.
// A server that does not answer within [Server.Answering] returns [engine.ErrDecline].
func (e *Engine) Verify(ctx context.Context, req engine.Request, suites []string) (engine.Result[edit.Finding], error) {
	out, err := e.verifying(ctx, req, suites)
	return out, e.unanswered(ctx, err)
}

// verifying is [Engine.Verify] before a missed deadline becomes a decline.
func (e *Engine) verifying(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	files, err := e.walk(req)
	if err != nil {
		return engine.Result[edit.Finding]{}, err
	}
	if len(files.Read) == 0 && len(files.Unread) == 0 {
		return engine.Result[edit.Finding]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()
	var docs []document
	for _, p := range files.Read {
		if err := ctx.Err(); err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		doc, err := e.load(ctx, held, p)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		docs = append(docs, doc)
	}
	ready := e.settle(ctx, held)

	by := time.Now().Add(reporting)
	var out []edit.Finding
	var waited bool
	for _, doc := range docs {
		reported, said, err := e.diagnostics(ctx, held, doc.path, by)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		waited = waited || !said
		for _, one := range reported {
			out = append(out, e.finding(ctx, held, one, doc, len(out)))
		}
	}

	covered, caveats := settled(ready)
	if waited || len(files.Unread) > 0 {
		covered = trust.ScopePartial
	}
	caveats = append(caveats, reasons(suites, waited)...)
	return engine.Result[edit.Finding]{
		Items:        out,
		Completeness: covered,
		Caveats:      append(caveats, unread(files.Unread)...),
	}, nil
}

// reasons returns the caveats of a verification beyond those of every answer: one for
// suites that the server ignored, and one for files that received no report.
func reasons(suites []string, waited bool) []trust.Caveat {
	var out []trust.Caveat
	if len(suites) > 0 {
		out = append(out, trust.Caveat{
			Code: trust.CaveatUnsupported,
			Note: "a language server runs one analysis, so the result ignores the suites named",
		})
	}
	if waited {
		out = append(out, trust.Caveat{
			Code: trust.CaveatIndexWarming,
			Note: "the server reported nothing about some files, so they may have findings",
		})
	}
	return out
}
