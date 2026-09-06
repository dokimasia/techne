// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"path"
	"slices"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Check reports what the server makes of content the workspace does not
// hold.
//
// # An unsaved buffer is how an editor asks this
//
// The protocol has no request that takes content, and it does not need
// one: a server analyses the buffers a client gives it, and a client
// gives it text nobody has written all day. So the content is shown as
// the buffer, the server is asked what is wrong with it, and the file is
// put back before this returns.
//
// That is what makes the write path's gate a compiler's answer rather
// than a parser's. A parse gate says the file is still the language it
// was; this says it still means something. A rename to a name already
// taken parses and does not compile, and nothing but a type checker
// tells the two apart.
//
// # Each finding carries the fix where there is one
//
// The same server that reports a fault offers the code actions that
// resolve it, and it is being asked already. One obvious fix is taken —
// the only one offered, or the one it marks preferred. Several plausible
// ones are left out rather than picked between.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	mine := make([]source.Path, 0, len(files))
	for p, content := range files {
		// A path with no content is one the change takes away. There is
		// nothing to analyse and nothing to object to.
		if content != nil && slices.Contains(e.declared.Extensions, path.Ext(string(p))) {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: nothing here is %s", engine.ErrDecline, e.declared.Language)
	}
	slices.Sort(mine)

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	// The server is holding text that is not on disk from here, and is
	// put back before this returns however it ends. A question arriving
	// meanwhile would otherwise put the files back underneath this one.
	e.showing.Lock()
	defer e.showing.Unlock()
	defer e.restore(ctx, held, mine)

	for _, p := range mine {
		if _, shown := e.sync(ctx, held, e.fullPath(p), files[p]); shown != nil {
			return engine.Result[edit.Finding]{}, shown
		}
	}
	// A server given several buffers at once analyses them together, so
	// this is waited on after all of them rather than per file.
	e.working.settle(ctx, e.settling())

	var out []edit.Finding
	settled := true
	for _, p := range mine {
		reported, said, err := e.diagnostics(ctx, held, p)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		settled = settled && said

		doc := texted(p, files[p])
		for _, one := range reported {
			out = append(out, e.finding(ctx, held, one, doc, len(out)))
		}
	}

	// A gate that says content is clean is the claim a caller acts on by
	// writing it, and a server that has not reported on every file it
	// was shown has not made that claim.
	//
	// Declined rather than answered short, so the parser beside this one
	// gets a turn: metals publishes nothing for a clean file, and a
	// half-answer from it would leave Scala with no gate at all where it
	// could still have had a grammar's.
	if !settled {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: %s said nothing about what it was shown", engine.ErrDecline, e.server.Name)
	}
	return engine.Result[edit.Finding]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Caveats:      []trust.Caveat{dynamic},
	}, nil
}

// diagnostics is what the server says about one file, and whether it
// said anything at all.
//
// Two ways a server reports and both are read. A server with a pull
// request answers about the buffer it is holding now, which is what a
// gate over unwritten content needs. One without publishes when it
// finishes, so the wait is bounded and reports whether it ended with an
// answer.
func (e *Engine) diagnostics(
	ctx context.Context,
	held *session,
	p source.Path,
) ([]protocol.Diagnostic, bool, error) {
	if held.capable.DiagnosticProvider != nil {
		reported, err := e.pull(ctx, held, p)
		return reported, true, err
	}
	reported, said := e.pushed.wait(ctx, uri.File(e.fullPath(p)), reporting)
	return reported, said, nil
}

// finding is one diagnostic in this vocabulary, with the change that
// resolves it where the server offers one obvious change.
//
// Fixes are fetched for faults alone, and only while there are few of
// them. A file that stopped compiling on line three reports a fault for
// most of what follows, and asking the server for a code action at each
// of them is a round trip apiece for a list nobody reads to the end.
func (e *Engine) finding(
	ctx context.Context,
	held *session,
	one protocol.Diagnostic,
	doc document,
	already int,
) edit.Finding {
	out := edit.Finding{Diagnostic: found(one, doc)}
	if out.Diagnostic.Severity < diag.SeverityError || already >= mendLimit {
		return out
	}
	out.Fix = e.mending(ctx, held, one, doc)
	return out
}

// mending is the change a server offers for one fault, and nothing where
// it offers none or several.
//
// Several plausible fixes is the case this leaves alone. A caller told
// there is one obvious change acts on it; told there are three, it has
// to choose, and choosing needs what the caller was asking this to save
// it from.
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
			// The fault this is about, so the server offers what
			// resolves it rather than everything it could do here.
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
		resolved, refused := held.asks.CodeActionResolve(ctx, action)
		if refused != nil || resolved == nil {
			return nil
		}
		made = resolved.Edit
	}
	if made == nil {
		return nil
	}

	// Against the content the server was shown rather than the file: a
	// gate judges what a change would produce, and that is what the
	// fault and the fix are both written in.
	changes, unreadable := e.changesAgainst(made, map[source.Path]document{doc.path: doc})
	if unreadable != nil || beyond(changes) != "" {
		return nil
	}
	return changes
}

// only returns the one obvious action among what was offered.
//
// One action is obvious. Several are obvious only where the server said
// which it prefers, which is what isPreferred is for and what an editor
// binds its one-key fix to.
func only(offered []protocol.CommandOrCodeAction) (*protocol.CodeAction, bool) {
	var single *protocol.CodeAction
	var count int

	for _, held := range offered {
		action, isAction := held.(*protocol.CodeAction)
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

// mendLimit caps how many faults in one check are looked up a fix for. A
// gate needs one fault to refuse, and the fixes are a convenience beside
// it.
const mendLimit = 8

// assert the engine serves the role it claims.
var _ engine.Checker = (*Engine)(nil)
