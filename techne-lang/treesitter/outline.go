// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"io/fs"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Outline reports what the files in a scope declare.
//
// The scope is one file or one directory. A directory is walked and
// every file this language claims is outlined; a file the language does
// not claim is skipped rather than refused, because a directory holding
// several languages is the normal case.
//
// Coverage is total: the walk reads every file in scope. What the
// answer is worth is still limited by the tier, and a caveat says so.
func (e *Engine) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	out, err := e.symbols(ctx, req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	return found(out), nil
}

// symbols reads every declaration in a scope. Outline returns them as
// they are; Search filters them.
func (e *Engine) symbols(ctx context.Context, req engine.Request) ([]sema.Symbol, error) {
	paths, err := lang.FilesIn(e.fsys, req.Scope, e.declared.Extensions)
	if err != nil {
		return nil, err
	}

	var out []sema.Symbol
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		content, readErr := fs.ReadFile(e.fsys, string(p))
		if readErr != nil {
			return nil, fmt.Errorf("treesitter: read %s: %w", p, readErr)
		}
		declared, outlineErr := e.declarations(p, content)
		if outlineErr != nil {
			return nil, outlineErr
		}
		out = append(out, declared...)
	}
	return out, nil
}

// found wraps symbols in the result every role at this tier returns.
//
// Coverage is total: the walk reads every file in scope. What the answer
// is worth is limited by the tier, and the caveat says so.
func found(items []sema.Symbol) engine.Result[sema.Symbol] {
	return engine.Result[sema.Symbol]{
		Items:        items,
		Completeness: trust.ScopeTotal,
		Caveats: []trust.Caveat{{
			Code: trust.CaveatDynamic,
			Note: "a parser matched text: a name resolved across files is coincidence",
		}},
	}
}

// declarations runs the tags query over one file.
//
// A match carries a definition capture and the name belonging to it. A
// match missing either is skipped: a query pattern that captures a name
// without saying what it declares describes no symbol.
func (e *Engine) declarations(p source.Path, content []byte) ([]sema.Symbol, error) {
	parser := ts.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(e.grammar.Language); err != nil {
		return nil, fmt.Errorf("treesitter: %s: %w", p, err)
	}

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("treesitter: %s: parser returned no tree", p)
	}
	defer tree.Close()

	cursor := ts.NewQueryCursor()
	defer cursor.Close()

	unit := source.Path(e.declared.Namespace(string(p)))
	names := e.tags.CaptureNames()

	var out []sema.Symbol
	matches := cursor.Matches(e.tags, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		kind, named, span, ok := read(match, names, p, content)
		if !ok {
			continue
		}
		out = append(out, sema.Symbol{
			ID:         sema.NewID(e.declared.Language, unit, named, kind),
			Name:       named,
			Kind:       kind,
			Language:   e.declared.Language,
			Span:       span,
			Visibility: e.declared.Visibility(named),
		})
	}
	return out, nil
}

// read pulls the kind, the name and the span out of one match, and
// reports whether the match declared a symbol at all.
func read(
	match *ts.QueryMatch,
	names []string,
	p source.Path,
	content []byte,
) (sema.Kind, string, source.Span, bool) {
	var (
		kind    sema.Kind
		named   string
		span    source.Span
		declare bool
	)
	for _, capture := range match.Captures {
		index := int(capture.Index)
		if index < 0 || index >= len(names) {
			continue
		}
		name := Capture(names[index])
		if name == Name {
			named = capture.Node.Utf8Text(content)
			continue
		}
		if k, declared := KindOf(name); declared {
			kind, declare = k, true
			span = spanOf(p, capture.Node)
		}
	}
	if !declare || named == "" {
		return 0, "", source.Span{}, false
	}
	return kind, named, span, true
}

// spanOf converts a node's range into a span. tree-sitter counts a row
// and a byte column, which is what source.Position holds.
func spanOf(p source.Path, n ts.Node) source.Span {
	start, end := n.StartPosition(), n.EndPosition()
	return source.Span{
		Path: p,
		Start: source.Position{
			Offset: int(n.StartByte()),
			Line:   int(start.Row),
			Column: int(start.Column),
		},
		End: source.Position{
			Offset: int(n.EndByte()),
			Line:   int(end.Row),
			Column: int(end.Column),
		},
	}
}

// assert the engine serves the role it claims.
var _ interface {
	engine.Engine
	engine.Outliner
} = (*Engine)(nil)
