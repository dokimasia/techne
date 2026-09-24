// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// refactorings are the code action kinds that the client declares, so a server returns code
// actions with a kind in place of bare commands.
var refactorings = []protocol.CodeActionKind{
	protocol.CodeActionKindEmpty,
	protocol.CodeActionKindRefactor,
	protocol.CodeActionKindRefactorExtract,
	protocol.CodeActionKindRefactorInline,
	protocol.CodeActionKindRefactorRewrite,
}

// extracting plans [edit.ExtractFunction]: it moves a run of whole lines into a new function
// with the name the caller chose.
//
// No server takes the name of the function it extracts. extracting asks the server for the
// code action that [Server.Extracts] names, shows the server the result as unsaved buffers,
// finds the declaration that the extraction added, and asks the server to rename it. The
// plan is the composition of both edits against the files on disk, one change per file. The
// buffers of the server contain the content on disk again when extracting returns.
//
// It returns [engine.ErrDecline] for a server that offers no such action, and a skipped
// result for lines in a file of another language.
func (e *Engine) extracting(
	ctx context.Context,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	fresh := strings.TrimSpace(args[edit.ArgNewName])
	switch {
	case fresh == "":
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s needs %s", engine.ErrRefuse, edit.ExtractFunction, edit.ArgNewName)
	case target.Kind != edit.TargetSpan || target.Span.Path == "":
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s names a run of lines, and this target names none", engine.ErrRefuse, edit.ExtractFunction)
	case !lang.Claims(string(target.Span.Path), e.declared.Extensions):
		return engine.Result[edit.Change]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	case !e.server.Extracts.Offered():
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s is declared with no code action that extracts a function", engine.ErrDecline, e.server.Name)
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()
	if !provides(held.capable.CodeActionProvider) {
		return engine.Result[edit.Change]{}, e.unsupported("textDocument/codeAction")
	}
	if !provides(held.capable.RenameProvider) {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s does not serve textDocument/rename, so it cannot name the function %s",
			engine.ErrDecline, e.server.Name, fresh)
	}

	p := target.Span.Path
	doc, err := e.open(ctx, held, p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	ready := e.settle(ctx, held)
	selected, within := selection(doc, target.Span)
	if !within {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s has %d lines, and the selection ends on line %d",
			engine.ErrRefuse, p, len(doc.at), target.Span.End.Line+1)
	}

	was, err := e.declarations(ctx, held, p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	lifted, err := e.lifting(ctx, held, p, selected)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	changes, err := e.changes(lifted, nil)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if outside := beyond(changes); outside != "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s: the extraction changes %s, which is outside the workspace",
			engine.ErrRefuse, e.server.Name, outside)
	}
	sealed, after, err := e.becoming(changes)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	// From here the buffers of the server differ from the disk until restore sends the disk
	// content back.
	e.showing.Lock()
	defer e.showing.Unlock()
	defer e.restore(ctx, held, slices.Collect(maps.Keys(after)))
	for shown, becomes := range after {
		if _, err = e.sync(ctx, held, e.fullPath(shown), becomes.content, stamp{}); err != nil {
			return engine.Result[edit.Change]{}, err
		}
	}

	now, err := e.declarations(ctx, held, p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	made, found := appeared(was, now)
	if !found {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s reports no new declaration after the extraction, so nothing can be named %s",
			engine.ErrRefuse, e.server.Name, fresh)
	}
	named, err := held.asks.Rename(ctx, &protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
		Position:     made,
		NewName:      fresh,
	})
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"lsp: %s: rename the extracted function: %w", e.server.Name, err)
	}
	renamed, err := e.changes(named, after)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	final, err := e.composed(sealed, after, renamed)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	out := make([]edit.Change, 0, len(final))
	for _, changed := range slices.Sorted(maps.Keys(final)) {
		if one, differs := differing(changed, sealed[changed], final[changed]); differs {
			out = append(out, one)
		}
	}
	if len(out) == 0 {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s computed no change for the lines", engine.ErrRefuse, e.server.Name)
	}
	covered, reaches, caveats := e.reached(ctx, held, p, true, ready, lang.Nowhere)
	return engine.Result[edit.Change]{
		Items:        out,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// lifting returns the workspace edit of the extraction the server offers for selected. It
// resolves an action that has no edit, and performs an action that has a command, whose edit
// the server offers through workspace/applyEdit.
func (e *Engine) lifting(
	ctx context.Context,
	held *session,
	p source.Path,
	selected protocol.Range,
) (*protocol.WorkspaceEdit, error) {
	wanted := e.server.Extracts
	only := []protocol.CodeActionKind{protocol.CodeActionKind(wanted.Kind)}
	if wanted.Kind == "" {
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
		return nil, fmt.Errorf("%w: %s offers no extraction of lines %d to %d of %s",
			engine.ErrRefuse, e.server.Name, selected.Start.Line+1, selected.End.Line+1, p)
	}
	if action.Edit != nil {
		return action.Edit, nil
	}
	if action.Data != nil && resolves(held.capable.CodeActionProvider) {
		// go.lsp.dev/protocol v1.0.1 writes into the action that CodeActionResolve sends, so
		// the request sends a copy.
		asked := *action
		resolved, err := held.asks.CodeActionResolve(ctx, &asked)
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: resolve %q: %w", e.server.Name, action.Title, err)
		}
		if resolved != nil {
			// gopls resolves an extraction to a command, and the edit exists once the
			// command runs.
			if resolved.Edit != nil {
				return resolved.Edit, nil
			}
			if runs(resolved, held.capable) {
				return e.commanded(ctx, held, resolved)
			}
		}
	}
	if runs(action, held.capable) {
		return e.commanded(ctx, held, action)
	}
	return nil, fmt.Errorf("%w: %s offers %q with no edit", engine.ErrDecline, e.server.Name, action.Title)
}

// runs reports whether action has a command and the server executes commands.
func runs(action *protocol.CodeAction, capable protocol.ServerCapabilities) bool {
	return action.Command.Command != "" && len(capable.ExecuteCommandProvider.Commands) > 0
}

// commanded performs the command of action with workspace/executeCommand, and returns the one
// edit that the server offered through workspace/applyEdit while the command ran. No edit
// returns [engine.ErrDecline]. Two or more edits return [engine.ErrRefuse], because each
// describes the files after the one before it.
func (e *Engine) commanded(
	ctx context.Context,
	held *session,
	action *protocol.CodeAction,
) (*protocol.WorkspaceEdit, error) {
	take := held.offering.arm()
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
		return nil, fmt.Errorf("%w: %s ran %q and offered no edit", engine.ErrDecline, e.server.Name, action.Title)
	}
	return nil, fmt.Errorf("%w: %s offered %d edits for %q, and a plan contains one",
		engine.ErrRefuse, e.server.Name, len(offered), action.Title)
}

// chosen returns the enabled code action of the wanted kind whose title matches the earliest
// title of wanted, and reports whether one exists. An action whose title matches none of
// them is chosen only when no action matches one. A bare command is skipped.
func chosen(offered []protocol.CommandOrCodeAction, wanted Refactor) (*protocol.CodeAction, bool) {
	var found *protocol.CodeAction
	rank := len(wanted.Titles) + 1
	for _, one := range offered {
		action, isAction := one.(*protocol.CodeAction)
		if !isAction || action.Disabled.Reason != "" || !kinded(action, wanted.Kind) {
			continue
		}
		if at := titled(action.Title, wanted.Titles); at < rank {
			found, rank = action, at
		}
	}
	return found, found != nil
}

// kinded reports whether the kind of action is wanted or below it: refactor.extract matches
// refactor.extract.function. An action without a kind matches every wanted kind, and every
// action matches an empty wanted kind.
func kinded(action *protocol.CodeAction, wanted string) bool {
	if wanted == "" || action.Kind == nil || *action.Kind == "" {
		return true
	}
	return strings.HasPrefix(string(*action.Kind), wanted)
}

// titled returns the index of the first of wanted that title contains, ignoring case, and
// len(wanted) when it contains none.
func titled(title string, wanted []string) int {
	lower := strings.ToLower(title)
	for i, one := range wanted {
		if strings.Contains(lower, strings.ToLower(one)) {
			return i
		}
	}
	return len(wanted)
}

// becoming reads each file of changes once, and returns its content on disk and the document
// of its content with the changes applied. A change that is not an edit returns
// [engine.ErrRefuse], because an extraction creates and moves no file.
func (e *Engine) becoming(changes []edit.Change) (map[source.Path][]byte, map[source.Path]document, error) {
	sealed := map[source.Path][]byte{}
	after := map[source.Path]document{}
	for _, c := range changes {
		if c.Kind != edit.ChangeEdit {
			return nil, nil, fmt.Errorf("%w: %s: the extraction creates, moves or deletes %s",
				engine.ErrRefuse, e.server.Name, c.Path)
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

// composed applies the edits of renamed to the content of after, and returns the content of
// every file either touches. It reads a file that only the rename touches from disk and adds
// its content to sealed.
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
			return nil, fmt.Errorf("%w: %s: the rename of the extracted function creates, moves or deletes %s",
				engine.ErrRefuse, e.server.Name, c.Path)
		}
		base, known := final[c.Path]
		if !known {
			doc, err := e.read(c.Path)
			if err != nil {
				return nil, err
			}
			base, sealed[c.Path] = doc.content, doc.content
		}
		content, err := edit.Apply(base, c.Edits)
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: %s: %w", e.server.Name, c.Path, err)
		}
		final[c.Path] = content
	}
	return final, nil
}

// placed is the name of a declaration and the protocol position of its name.
type placed struct {
	name string
	at   protocol.Position
}

// declarations returns the name and the name position of every declaration in the buffer of
// the server for p. The positions are in protocol coordinates, because the buffer may differ
// from the file on disk.
func (e *Engine) declarations(ctx context.Context, held *session, p source.Path) ([]placed, error) {
	if !provides(held.capable.DocumentSymbolProvider) {
		return nil, e.unsupported("textDocument/documentSymbol")
	}
	answered, err := held.asks.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: symbols of %s: %w", e.server.Name, p, err)
	}
	var out []placed
	switch reported := answered.(type) {
	case protocol.DocumentSymbolSlice:
		placing(&out, reported)
	case protocol.SymbolInformationSlice:
		for _, one := range reported {
			out = append(out, placed{name: trimmed(one.Name), at: one.Location.Range.Start})
		}
	}
	return out, nil
}

// placing appends every declaration of a DocumentSymbol tree to into, depth first. A server
// reports an extracted method inside the type it belongs to.
func placing(into *[]placed, reported []protocol.DocumentSymbol) {
	for _, one := range reported {
		*into = append(*into, placed{name: trimmed(one.Name), at: one.SelectionRange.Start})
		placing(into, one.Children)
	}
}

// appeared returns the name position of the first declaration of now whose name occurs more
// often in now than in was, and reports whether one does. Counting names finds the new
// declaration also when the file declared its name before.
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

// selection returns the protocol range from character 0 of the first line of span to the end
// of its last line, without the line ending, and reports whether both lines are in doc.
func selection(doc document, span source.Span) (protocol.Range, bool) {
	first, last := span.Start.Line, span.End.Line
	if first < 0 || last < first || last >= len(doc.at) {
		return protocol.Range{}, false
	}
	start, end := doc.bounds(last)
	return protocol.Range{
		Start: protocol.Position{Line: uint32(first)},
		End:   protocol.Position{Line: uint32(last), Character: doc.unitsFor(last, end-start)},
	}, true
}
