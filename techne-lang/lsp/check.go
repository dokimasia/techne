// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"slices"
	"time"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// mendLimit is the number of error findings in one call for which the engine requests a fix.
const mendLimit = 8

// unchecked is the caveat on a check by a server that does not return workspace diagnostics.
var unchecked = trust.Caveat{
	Code: trust.CaveatDependents,
	Note: "the server checked the changed files and not the files that depend on them",
}

// Check returns the findings of the server for content that is not on disk, such as the
// projection of a change before the write path writes it. A path of another language and a
// path with nil content, which a change deletes, are left out. Check takes these steps:
//
//  1. Show the server each file of files as an unsaved buffer.
//  2. Wait for the server to settle.
//  3. Collect the diagnostics of each file.
//  4. Send the server the content on disk again, also when a step fails.
//
// A server that returns workspace diagnostics also reports the files that depend on the
// changed files, and their findings follow the findings of the changed files. For any other
// server the result contains the caveat [trust.CaveatDependents]. A server whose declaration
// names what it leaves [Server.Unchecked] adds a [trust.CaveatPartialCheck] caveat.
//
// An error finding contains a fix when the server offers one quick fix for it, or marks one of
// two or more as preferred. Check returns [engine.ErrDecline] in three cases, so the next
// engine checks the content:
//
//   - files contain no path of the language.
//   - The server has not settled.
//   - The server reported nothing about a file.
//   - The server did not answer within [Server.Answering].
func (e *Engine) Check(ctx context.Context, files map[source.Path][]byte) (engine.Result[edit.Finding], error) {
	out, err := e.checking(ctx, files)
	return out, e.unanswered(ctx, err)
}

// checking is [Engine.Check] before a missed deadline becomes a decline.
func (e *Engine) checking(ctx context.Context, files map[source.Path][]byte) (engine.Result[edit.Finding], error) {
	var mine []source.Path
	for p, content := range files {
		if content != nil && lang.Claims(string(p), e.declared.Extensions) {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: no changed file is %s", engine.ErrDecline, e.declared.Language)
	}
	slices.Sort(mine)

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()

	e.showing.Lock()
	defer e.showing.Unlock()
	defer e.restore(ctx, held, mine)
	for _, p := range mine {
		if _, err := e.sync(ctx, held, e.fullPath(p), files[p], stamp{}); err != nil {
			return engine.Result[edit.Finding]{}, err
		}
	}
	if !e.settle(ctx, held) {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: %s is still loading the workspace", engine.ErrDecline, e.server.Name)
	}

	by := time.Now().Add(reporting)
	var out []edit.Finding
	for _, p := range mine {
		reported, said, err := e.diagnostics(ctx, held, p, by)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		if !said {
			return engine.Result[edit.Finding]{}, fmt.Errorf(
				"%w: %s reported nothing about %s", engine.ErrDecline, e.server.Name, p)
		}
		doc := texted(p, files[p])
		for _, one := range reported {
			out = append(out, e.finding(ctx, held, one, doc, len(out)))
		}
	}

	caveats := []trust.Caveat{dynamic}
	if e.server.Unchecked != "" {
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatPartialCheck,
			Note: e.server.Name + " does not check " + e.server.Unchecked + ", which the compiler checks, " +
				"so the build can still refuse a change that this check passes",
		})
	}
	if !workspaceWide(held.capable.DiagnosticProvider) {
		return engine.Result[edit.Finding]{
			Items: out, Completeness: trust.ScopeTotal, Caveats: append(caveats, unchecked),
		}, nil
	}
	others, failed := e.dependents(ctx, held, mine)
	switch {
	case failed != nil && ctx.Err() != nil:
		return engine.Result[edit.Finding]{}, failed
	case failed != nil:
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatDependents,
			Note: "the files that depend on the changed files were not checked: " + failed.Error(),
		})
	default:
		out = append(out, others...)
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: trust.ScopeTotal, Caveats: caveats}, nil
}

// finding converts one diagnostic of doc into a finding. An error finding contains the fix
// from [Engine.mending] while fewer than [mendLimit] findings precede it in the call.
func (e *Engine) finding(
	ctx context.Context,
	held *session,
	one protocol.Diagnostic,
	doc document,
	preceding int,
) edit.Finding {
	out := edit.Finding{Diagnostic: found(one, doc)}
	if out.Diagnostic.Severity != diag.SeverityError || preceding >= mendLimit {
		return out
	}
	out.Fix = e.mending(ctx, held, one, doc)
	return out
}

// mending returns the changes of the one quick fix that the server offers for a diagnostic
// of doc, resolved when the server resolves code actions. The ranges of the fix convert against
// doc, the content the server was shown. mending returns nil in these cases:
//
//   - The server does not offer a fix.
//   - The server offers two or more fixes without a preferred one.
//   - The server refuses the request.
//   - The fix changes a file outside the workspace.
func (e *Engine) mending(
	ctx context.Context,
	held *session,
	one protocol.Diagnostic,
	doc document,
) []edit.Change {
	if !provides(held.capable.CodeActionProvider) {
		return nil
	}
	offered, err := held.asks.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Range:        one.Range,
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{one},
			Only:        []protocol.CodeActionKind{protocol.CodeActionKindQuickFix},
			TriggerKind: protocol.CodeActionTriggerKindAutomatic,
		},
	})
	if err != nil {
		return nil
	}
	action, obvious := only(offered)
	if !obvious {
		return nil
	}
	made := action.Edit
	if made == nil && action.Data != nil && resolves(held.capable.CodeActionProvider) {
		// go.lsp.dev/protocol v1.0.1 writes into the action that CodeActionResolve sends, so
		// the request sends a copy.
		asked := *action
		resolved, failed := held.asks.CodeActionResolve(ctx, &asked)
		if failed != nil || resolved == nil {
			return nil
		}
		made = resolved.Edit
	}
	if made == nil {
		return nil
	}
	changes, err := e.changes(made, map[source.Path]document{doc.path: doc})
	if err != nil || beyond(changes) != "" {
		return nil
	}
	return changes
}

// only returns the one enabled code action of offered, or the first enabled action that the
// server marks as preferred, and reports whether there is one. Two or more enabled actions
// without a preferred one return false. A bare command is skipped.
func only(offered []protocol.CommandOrCodeAction) (*protocol.CodeAction, bool) {
	var single *protocol.CodeAction
	var count int
	for _, one := range offered {
		action, isAction := one.(*protocol.CodeAction)
		if !isAction || action.Disabled.Reason != "" {
			continue
		}
		if action.IsPreferred != nil && *action.IsPreferred {
			return action, true
		}
		single, count = action, count+1
	}
	return single, count == 1
}
