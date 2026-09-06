// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// refactorings are the code action kinds techne asks for, declared to a
// server so it answers with actions rather than with bare commands.
var refactorings = []protocol.CodeActionKind{
	protocol.CodeActionKindEmpty,
	protocol.CodeActionKindRefactor,
	protocol.CodeActionKindRefactorExtract,
	protocol.CodeActionKindRefactorInline,
	protocol.CodeActionKindRefactorRewrite,
}

// extracting lifts a run of lines into a function the caller names.
//
// # It is two refactorings, because no server offers one
//
// Every server names the function it makes: newFunction, fun_name,
// getWeighted. None takes a name, because in an editor the name is what
// the user types into the box that opens afterwards. techne has no box,
// and the caller has already said what it wants the function called.
//
// So the extraction is computed, the result is given to the server as an
// unsaved buffer, the declaration that appeared in it is found, and that
// is renamed — which is what the editor's box does. The two results are
// composed into one change against the file as it is on disk, so the
// write path sees one operation and gates it once.
//
// # Nothing is written to get there
//
// The buffer the server is shown is a notification, not a file: an
// editor holds unsaved text this way all day. The server is put back on
// the file's real content before this returns, whether or not the plan
// is ever applied.
func (e *Engine) extracting(
	ctx context.Context,
	_ engine.Request,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	const op = edit.ExtractFunction

	fresh := strings.TrimSpace(args[edit.ArgNewName])
	if fresh == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s needs %s", engine.ErrRefuse, op, edit.ArgNewName)
	}
	if target.Kind != edit.TargetSpan || target.Span.Path == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s is pointed at a run of lines, and this names none",
			engine.ErrRefuse, op)
	}
	if !e.server.Extracts.Offered() {
		// Declared per server because it was established per server. A
		// server nobody probed is not one that refuses; it is one nobody
		// has asked, and saying so lets a parser beside it answer.
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s offers no refactoring that lifts lines into a function",
			engine.ErrDecline, e.server.Name)
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !provides(held.capable.CodeActionProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/codeAction")
	}
	if !provides(held.capable.RenameProvider) {
		// The extraction alone would produce a function called whatever
		// the server calls one, which is not what was asked for.
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s cannot rename, so it cannot name the function %s",
			engine.ErrDecline, e.server.Name, fresh)
	}

	p := target.Span.Path
	if opened := e.open(ctx, held, p); opened != nil {
		return engine.Result[edit.Change]{}, opened
	}
	// An extraction computed against a half-loaded workspace works out
	// the parameters from the types the server has bound so far.
	e.working.settle(ctx, e.settling())

	doc, err := e.read(p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	selected, within := selection(doc, target.Span)
	if !within {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s has %d lines, and the selection runs to %d",
			engine.ErrRefuse, p, len(doc.at), target.Span.End.Line+1)
	}

	// What the file declares before anything is extracted, so the
	// declaration that appears afterwards can be told from the ones that
	// were already there.
	was, err := e.declarations(ctx, held, p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	lifted, err := e.lifting(ctx, held, p, selected)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	changes, err := e.changes(lifted)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if outside := beyond(changes); outside != "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: the extraction reaches %s, which is outside the workspace",
			engine.ErrRefuse, e.server.Name, outside)
	}

	sealed, after, err := e.becoming(changes)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	// From here the server is holding text that is not on disk, and is
	// put back before this returns however it ends. A server left
	// holding a buffer nobody wrote reports diagnostics about code that
	// is nowhere, and a question arriving meanwhile would put the file
	// back underneath this one.
	e.showing.Lock()
	defer e.showing.Unlock()
	defer e.restore(ctx, held, slices.Collect(maps.Keys(after)))
	for path, becomes := range after {
		if _, shown := e.sync(ctx, held, e.fullPath(path), becomes.content); shown != nil {
			return engine.Result[edit.Change]{}, shown
		}
	}

	now, err := e.declarations(ctx, held, p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	made, found := appeared(was, now)
	if !found {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s extracted something it does not report as a declaration, "+
				"so there is nothing to name %s", engine.ErrRefuse, e.server.Name, fresh)
	}

	named, err := held.asks.Rename(ctx, &protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
		Position:     made,
		NewName:      fresh,
	})
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"lsp: %s: naming the extracted function: %w", e.server.Name, err)
	}
	renamed, err := e.changesAgainst(named, after)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	final, err := e.composed(sealed, after, renamed)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	out := make([]edit.Change, 0, len(final))
	for _, path := range slices.Sorted(maps.Keys(final)) {
		if one, differs := differing(path, sealed[path], final[path]); differs {
			out = append(out, one)
		}
	}
	if len(out) == 0 {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s worked out no change for those lines", engine.ErrRefuse, e.server.Name)
	}

	// The server computed an extraction and named what it made, which
	// is as clear a view of the file as it can show.
	covered, reaches, caveats := e.reached(ctx, held, p, true)
	return engine.Result[edit.Change]{
		Items:        out,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// lifting is the workspace edit behind the server's own extraction.
func (e *Engine) lifting(
	ctx context.Context,
	held *session,
	p source.Path,
	selected protocol.Range,
) (*protocol.WorkspaceEdit, error) {
	wanted := e.server.Extracts
	only := []protocol.CodeActionKind{protocol.CodeActionKind(wanted.Kind)}
	if wanted.Kind == "" {
		// A server that sets no kind is not one that filters on it
		// either, and asking for the base kind is what narrows the work
		// for the ones that do.
		only = []protocol.CodeActionKind{protocol.CodeActionKindRefactor}
	}

	offered, err := held.asks.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
		Range:        selected,
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{},
			Only:        only,
			TriggerKind: protocol.CodeActionTriggerKindInvoked,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: code action: %w", e.server.Name, err)
	}

	action, chose := chosen(offered, wanted)
	if !chose {
		return nil, fmt.Errorf(
			"%w: %s offers nothing that lifts those lines into a function: "+
				"a selection has to be whole statements", engine.ErrRefuse, e.server.Name)
	}
	if action.Edit != nil {
		return action.Edit, nil
	}
	if action.Data != nil && resolves(held.capable.CodeActionProvider) {
		// Most servers compute the edit only when asked. Working one out
		// for every action in a menu nobody opened is what they avoid.
		resolved, err := held.asks.CodeActionResolve(ctx, action)
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: resolve %q: %w", e.server.Name, action.Title, err)
		}
		if resolved != nil && resolved.Edit != nil {
			return resolved.Edit, nil
		}
	}
	if action.Command.Command != "" && len(held.capable.ExecuteCommandProvider.Commands) > 0 {
		return e.commanded(ctx, held, action)
	}
	return nil, fmt.Errorf(
		"%w: %s offers %q and hands back no edit for it",
		engine.ErrDecline, e.server.Name, action.Title)
}

// commanded runs a code action that is a command rather than an edit,
// and takes the edit the server offers to apply.
//
// typescript-language-server exposes every refactoring this way: it does
// the work when the command is run and sends the result to the client to
// apply. There is nothing else to take it from, and taken here it is
// still not applied — it becomes the plan, and goes through the gate
// every other change goes through.
func (e *Engine) commanded(
	ctx context.Context,
	held *session,
	action *protocol.CodeAction,
) (*protocol.WorkspaceEdit, error) {
	take := e.offering.arm()
	_, err := held.asks.ExecuteCommand(ctx, &protocol.ExecuteCommandParams{
		Command:   action.Command.Command,
		Arguments: action.Command.Arguments,
	})
	offered := take()

	if err != nil {
		return nil, fmt.Errorf("lsp: %s: %s: %w", e.server.Name, action.Command.Command, err)
	}
	switch len(offered) {
	case 1:
		return offered[0], nil
	case 0:
		return nil, fmt.Errorf(
			"%w: %s ran %q and offered no edit",
			engine.ErrDecline, e.server.Name, action.Title)
	default:
		// Several edits computed against each other's results, which is
		// what applying them one at a time would mean. Nothing here
		// applies anything, so there is no order to put them in.
		return nil, fmt.Errorf(
			"%w: %s offered %d separate edits for %q, which describe one another's results",
			engine.ErrRefuse, e.server.Name, len(offered), action.Title)
	}
}

// asking is what a server offered to apply while techne was asking it to
// compute something.
//
// Armed for exactly as long as one command is running, and one at a
// time: two extractions in flight would otherwise take each other's
// edits. Everything a server offers outside that window is refused,
// which is what [answers.ApplyEdit] is for.
type asking struct {
	// one holds the window open for a single command.
	one sync.Mutex
	// mu guards the rest against the goroutine reading the connection.
	mu    sync.Mutex
	armed bool
	held  []*protocol.WorkspaceEdit
}

// arm opens the window, and returns what closes it and hands back what
// arrived.
func (a *asking) arm() func() []*protocol.WorkspaceEdit {
	a.one.Lock()
	a.mu.Lock()
	a.armed, a.held = true, nil
	a.mu.Unlock()

	return func() []*protocol.WorkspaceEdit {
		a.mu.Lock()
		held := a.held
		a.armed, a.held = false, nil
		a.mu.Unlock()
		a.one.Unlock()
		return held
	}
}

// offered keeps an edit if one was asked for, and reports whether it
// was.
func (a *asking) offered(held *protocol.WorkspaceEdit) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.armed {
		return false
	}
	a.held = append(a.held, held)
	return true
}

// chosen picks the action the language module named.
//
// A bare command is passed over: it carries no kind and nothing this can
// resolve, and a client that declared it reads code action literals is
// not sent one by a server that has any.
func chosen(offered []protocol.CommandOrCodeAction, wanted Refactor) (*protocol.CodeAction, bool) {
	var found *protocol.CodeAction
	rank := len(wanted.Titles) + 1

	for _, one := range offered {
		action, isAction := one.(*protocol.CodeAction)
		if !isAction || action.Disabled.Reason != "" || !kinded(action, wanted.Kind) {
			continue
		}
		at := titled(action.Title, wanted.Titles)
		if at < rank {
			found, rank = action, at
		}
	}
	return found, found != nil
}

// kinded reports whether an action is of the kind that was asked for.
//
// Kinds are hierarchical, so refactor.extract covers
// refactor.extract.function. An action carrying no kind matches
// whatever was asked for: the server that sets none also offers nothing
// but what the request narrowed it to.
func kinded(action *protocol.CodeAction, wanted string) bool {
	if wanted == "" || action.Kind == nil || *action.Kind == "" {
		return true
	}
	return strings.HasPrefix(string(*action.Kind), wanted)
}

// titled ranks a title against the wordings a module preferred, best
// first. A title matching none ranks last but is still a candidate: a
// server wording an action differently in a context nobody probed is
// still offering the refactoring that was asked for.
func titled(title string, wanted []string) int {
	held := strings.ToLower(title)
	for i, one := range wanted {
		if strings.Contains(held, strings.ToLower(one)) {
			return i
		}
	}
	return len(wanted)
}

// resolves reports whether a server computes a code action's edit only
// when asked for it.
func resolves(held any) bool {
	options, declared := held.(*protocol.CodeActionOptions)
	return declared && options.ResolveProvider != nil && *options.ResolveProvider
}

// becoming reads the files a change touches and works out what each one
// would hold, keeping both.
//
// The originals are what the composed edits are measured against and the
// results are what the server is shown, and they have to come from one
// read: a file that moved between them would produce a plan describing
// neither.
func (e *Engine) becoming(
	changes []edit.Change,
) (map[source.Path][]byte, map[source.Path]document, error) {
	sealed := map[source.Path][]byte{}
	after := map[source.Path]document{}

	for _, c := range changes {
		if c.Kind != edit.ChangeEdit {
			return nil, nil, fmt.Errorf(
				"%w: %s: lifting lines into a function does not create or move files, "+
					"and this one does", engine.ErrRefuse, e.server.Name)
		}
		doc, err := e.read(c.Path)
		if err != nil {
			return nil, nil, err
		}
		content, err := edit.Apply(doc.content, c.Edits)
		if err != nil {
			return nil, nil, fmt.Errorf("lsp: %s: %s: %w", e.server.Name, c.Path, err)
		}
		sealed[c.Path] = doc.content
		after[c.Path] = texted(c.Path, content)
	}
	return sealed, after, nil
}

// composed applies the naming edits on top of the extraction's result.
//
// A file the extraction did not touch is read here, so it is measured
// against its own content rather than against nothing: taking the
// missing entry for an empty file would make the edit an insertion of
// the whole file in front of itself.
func (e *Engine) composed(
	sealed map[source.Path][]byte,
	after map[source.Path]document,
	renamed []edit.Change,
) (map[source.Path][]byte, error) {
	final := map[source.Path][]byte{}
	for p, doc := range after {
		final[p] = doc.content
	}

	for _, c := range renamed {
		if c.Kind != edit.ChangeEdit {
			return nil, fmt.Errorf(
				"%w: naming the extracted function moved a file", engine.ErrRefuse)
		}
		base, held := final[c.Path]
		if !held {
			// A file the extraction did not touch. Its original is its
			// current content, and both are what is on disk.
			doc, err := e.read(c.Path)
			if err != nil {
				return nil, err
			}
			base, sealed[c.Path] = doc.content, doc.content
		}
		content, err := edit.Apply(base, c.Edits)
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: %w", c.Path, err)
		}
		final[c.Path] = content
	}
	return final, nil
}

// restore puts the server back on what the files hold on disk.
//
// On a context of its own: the call that showed the server the unsaved
// buffer may be cancelled by the time this runs, and a server left
// holding text nobody wrote reports diagnostics about code that is
// nowhere.
func (e *Engine) restore(ctx context.Context, held *session, paths []source.Path) {
	back := context.WithoutCancel(ctx)
	for _, p := range paths {
		_ = e.open(back, held, p)
	}
}

// declarations is what a file declares and where each name is written,
// read off the server rather than converted.
//
// The positions stay in the protocol's coordinates because what this is
// for is comparing two outlines of one file, and the second is of text
// that is not on disk to convert against.
func (e *Engine) declarations(
	ctx context.Context,
	held *session,
	p source.Path,
) ([]placed, error) {
	if !provides(held.capable.DocumentSymbolProvider) {
		return nil, e.unsupported("textDocument/documentSymbol")
	}
	answered, err := held.asks.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: outline %s: %w", e.server.Name, p, err)
	}

	var out []placed
	switch reported := answered.(type) {
	case protocol.DocumentSymbolSlice:
		nested(&out, reported)
	case protocol.SymbolInformationSlice:
		for _, one := range reported {
			out = append(out, placed{
				name: trimmed(one.Name), at: one.Location.Range.Start,
			})
		}
	}
	return out, nil
}

// placed is a declaration's name and where its own name is written.
type placed struct {
	name string
	at   protocol.Position
}

// nested walks the tree, because a server puts an extracted method under
// the type it belongs to.
func nested(into *[]placed, held []protocol.DocumentSymbol) {
	for _, one := range held {
		*into = append(*into, placed{
			name: trimmed(one.Name), at: one.SelectionRange.Start,
		})
		nested(into, one.Children)
	}
}

// appeared is where the declaration that was not there before is
// written.
//
// By name and by count, so a language that already declares something of
// that name is not mistaken for one that declares nothing: what is
// looked for is a name the file now declares once more often than it
// did.
func appeared(was, now []placed) (protocol.Position, bool) {
	before := map[string]int{}
	for _, one := range was {
		before[one.name]++
	}
	seen := map[string]int{}
	for _, one := range now {
		seen[one.name]++
		if seen[one.name] > before[one.name] {
			return one.at, true
		}
	}
	return protocol.Position{}, false
}

// selection is a run of lines as the protocol writes a range.
//
// A caller counts lines off an editor and sends no byte offsets, because
// working them out needs the file. The range runs from the first
// character of the first line to the last character of the last, which
// is what selecting whole lines means and what a server needs to see
// statements rather than a fragment of one.
func selection(doc document, span source.Span) (protocol.Range, bool) {
	first, last := span.Start.Line, span.End.Line
	if first < 0 || last < first || last >= len(doc.at) {
		return protocol.Range{}, false
	}
	end := doc.line(uint32(last))
	return protocol.Range{
		Start: protocol.Position{Line: uint32(first)},
		End: protocol.Position{
			Line:      uint32(last),
			Character: unitsFor(end, len(end)),
		},
	}, true
}

// differing is one file's change as the smallest whole-line range that
// covers everything that moved.
//
// Composing two refactorings gives a file's new content rather than a
// list of edits, so the edit is worked back out of the two texts. Whole
// lines because a change is read as a diff: an edit that began mid-line
// would print half a line against half another.
func differing(p source.Path, before, after []byte) (edit.Change, bool) {
	if bytes.Equal(before, after) {
		return edit.Change{}, false
	}

	head := 0
	for head < len(before) && head < len(after) && before[head] == after[head] {
		head++
	}
	for head > 0 && before[head-1] != '\n' {
		head--
	}

	tail := 0
	for tail < len(before)-head && tail < len(after)-head &&
		before[len(before)-1-tail] == after[len(after)-1-tail] {
		tail++
	}
	for tail > 0 && before[len(before)-tail-1] != '\n' {
		tail--
	}

	return edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{{
		Span: source.Span{
			Path:  p,
			Start: source.Position{Offset: head},
			End:   source.Position{Offset: len(before) - tail},
		},
		New: string(after[head : len(after)-tail]),
	}}}, true
}
