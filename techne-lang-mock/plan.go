// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Plan computes the edits an operation would need.
//
// Three of them, and each is a real rewrite of real bytes: a rename
// moves every use as well as the declaration, documenting writes a
// comment above it, and moving a file moves it. What makes this worth
// having is the first: a rename that rewrites references is the
// operation the policy refuses on weak evidence, and until something
// could plan one there was nothing for that refusal to be tested
// against.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	held, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	switch op {
	case edit.RenameSymbol:
		return e.renaming(held, target, args[edit.ArgNewName])
	case edit.DocumentSymbol:
		return e.documenting(held, target, args[edit.ArgDoc])
	case edit.MoveFile:
		return e.moving(target, args[edit.ArgDestination])
	default:
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: this language has no planner for %s", engine.ErrDecline, op)
	}
}

// renaming rewrites a declaration's name and every use of it.
//
// Every use, which is the whole point: a rename that moved nine of ten
// would leave code that still parses and means something else, and the
// policy admitting this is what says the evidence was strong enough to
// claim there is no tenth.
func (e *Engine) renaming(
	held workspace,
	target edit.Target,
	name string,
) (engine.Result[edit.Change], error) {
	one, found := pointed(held, target)
	if !found {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: nothing is declared there", engine.ErrRefuse)
	}
	if name == one.Name {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: it is already called %s", engine.ErrRefuse, name)
	}

	at := map[source.Path][]edit.TextEdit{}
	for _, p := range slices.Sorted(maps.Keys(held.lines)) {
		for _, line := range held.lines[p] {
			if line.Name != one.Name && line.Uses != one.Name {
				continue
			}
			at[p] = append(at[p], edit.TextEdit{Span: line.At, New: name})
		}
	}
	out := changes(at)
	out.Completeness = e.coverage
	return out, nil
}

// documenting writes a comment above a declaration, replacing whatever
// documentation is there.
func (e *Engine) documenting(
	held workspace,
	target edit.Target,
	text string,
) (engine.Result[edit.Change], error) {
	one, found := pointed(held, target)
	if !found {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: nothing is declared there", engine.ErrRefuse)
	}
	if strings.TrimSpace(text) == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: there is nothing to write", engine.ErrRefuse)
	}

	p := one.Span.Path
	from, to := above(held.content[p], one)
	indent := strings.Repeat(" ", one.Span.Start.Column)

	var written []string
	for line := range strings.SplitSeq(text, "\n") {
		written = append(written, indent+documents+" "+line)
	}
	out := changes(map[source.Path][]edit.TextEdit{p: {{
		Span: source.Span{
			Path:  p,
			Start: source.Position{Offset: from},
			End:   source.Position{Offset: to},
		},
		New: strings.Join(written, "\n") + "\n",
	}}})
	out.Completeness = e.coverage
	return out, nil
}

// above is the byte range the documentation for a declaration occupies,
// or an empty range at the start of its line where there is none.
//
// Replacing rather than adding to: a declaration carrying two comments
// is one this language's reader takes only the second of.
func above(content []byte, of sema.Symbol) (from, to int) {
	start := of.Span.Start.Offset - of.Span.Start.Column
	from = start
	for from > 0 {
		line := from - 1
		for line > 0 && content[line-1] != '\n' {
			line--
		}
		if !strings.HasPrefix(strings.TrimLeft(string(content[line:from-1]), " "), documents) {
			break
		}
		from = line
	}
	return from, start
}

// moving takes a file somewhere else. Nothing refers to a file in this
// language, so the move is the whole change.
func (e *Engine) moving(target edit.Target, to string) (engine.Result[edit.Change], error) {
	if to == "" {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: there is nowhere to move it to", engine.ErrRefuse)
	}
	return engine.Result[edit.Change]{
		Items: []edit.Change{{
			Kind: edit.ChangeMove, Path: target.Path, To: source.Path(to),
		}},
		Completeness: e.coverage,
	}, nil
}

// pointed is the declaration a target names.
func pointed(held workspace, target edit.Target) (sema.Symbol, bool) {
	switch target.Kind {
	case edit.TargetSpan:
		for _, one := range held.symbols {
			if one.Span.Path == target.Span.Path &&
				one.Span.Start.Offset == target.Span.Start.Offset {
				return one, true
			}
		}
	case edit.TargetSymbol:
		return byID(held.symbols, target.Symbol)
	case edit.TargetUnset, edit.TargetFile:
	}
	return sema.Symbol{}, false
}

// changes turns per-file edits into the changes a plan carries, sorted
// as the write path requires: it walks an edit list once and never looks
// back, so an unsorted list writes the wrong bytes.
func changes(at map[source.Path][]edit.TextEdit) engine.Result[edit.Change] {
	var out []edit.Change
	for _, p := range slices.Sorted(maps.Keys(at)) {
		edits := at[p]
		slices.SortFunc(edits, func(a, b edit.TextEdit) int {
			return a.Span.Start.Offset - b.Span.Start.Offset
		})
		out = append(out, edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: edits})
	}
	return engine.Result[edit.Change]{Items: out}
}

// assert the engine serves every role its package comment claims.
var _ interface {
	engine.Outliner
	engine.Searcher
	engine.Resolver
	engine.Relator
	engine.Planner
	engine.Checker
	engine.Verifier
} = (*Engine)(nil)
