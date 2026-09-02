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
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
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
	// Where each symbol's declaration began, so a statement binding
	// several names can be told from one binding a single name.
	var from []uint
	at := map[int]int{}
	walk := tree.RootNode().Walk()
	defer walk.Close()

	matches := cursor.Matches(e.tags, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		kind, node, named, span, ok := read(match, names, p)
		if !ok {
			continue
		}

		for _, one := range declared(node, named, walk, content) {
			one.text = unquote(one.text)
			if e.declared.Blank[one.text] {
				continue
			}
			// One declaration can match a general pattern and a specific
			// one. Both name the same identifier, so where the name sits
			// is what tells them apart from two declarations that happen
			// to share a name.
			if seen, already := at[one.at]; already {
				if Outranks(kind, out[seen].Kind) {
					out[seen].Kind = kind
					out[seen].ID = sema.NewID(e.declared.Language, unit, one.text, kind)
				}
				continue
			}

			marks := annotations(node, content, p)
			signed := signature(node, content, marks, kind, e.declared.Comment)
			if kind == sema.KindField {
				marks = append(marks, tags(node, content, p)...)
			}

			at[one.at] = len(out)
			from = append(from, node.StartByte())
			out = append(out, sema.Symbol{
				ID:          sema.NewID(e.declared.Language, unit, one.text, kind),
				Name:        one.text,
				Kind:        kind,
				Language:    e.declared.Language,
				Span:        span,
				Visibility:  e.declared.Visibility(one.text),
				Modifiers:   modifiers(node, content),
				Annotations: marks,
				Signature:   signed,
				Doc:         documentation(node, content, e.declared.Comment, kind),
				Snippet:     snippetOf(content, span),
			})
		}
	}
	named(out, from)
	Parents(out)
	return out, nil
}

// named replaces the signature of every symbol that shares its
// declaration with another.
//
// One statement can bind many names: Python writes
// `(A, B, C) = range(3)` and Go writes `const a, b = 1, 2`. The
// statement is the signature of none of them on its own, and reporting
// it once per name says the whole of it three times.
func named(symbols []sema.Symbol, from []uint) {
	shared := map[uint]int{}
	for _, at := range from {
		shared[at]++
	}
	for i := range symbols {
		if i < len(from) && shared[from[i]] > 1 {
			symbols[i].Signature = symbols[i].Name
		}
	}
}

// unquote strips the quotes from a name a grammar gives as a string
// literal. An import names its target that way in most languages, and
// the quotes are punctuation rather than part of the name.
func unquote(name string) string {
	if len(name) >= 2 {
		if first, last := name[0], name[len(name)-1]; first == last {
			switch first {
			case '"', '\'', '`':
				return name[1 : len(name)-1]
			}
		}
	}
	return name
}

// identifier is one name a declaration binds, and where it sits.
type identifier struct {
	text string
	at   int
}

// declared returns every name one declaration binds.
//
// A declaring node that names itself is believed over the capture, for
// two reasons. Go writes `const a, b = 1, 2` as one spec carrying two
// name fields, and a pattern binds a capture once, so the second name is
// unreachable from the query. And an embedded field has no name field at
// all: `struct { FileHeader }` declares FileHeader by its type, so the
// query captures the type and this leaves it alone.
//
// The capture is the fallback for the many patterns whose name is not a
// name field of the declaring node, as Python's assignment writes it
// under left and Java's field declaration under declarator.
func declared(node, named *ts.Node, walk *ts.TreeCursor, content []byte) []identifier {
	if fields := node.ChildrenByFieldName(string(FieldNameName), walk); len(fields) > 1 {
		out := make([]identifier, 0, len(fields))
		for _, field := range fields {
			// ChildrenByFieldName hands back the separators between the
			// fields as well as the fields, so `a, b` arrives as three
			// nodes. Only the named ones declare anything.
			if !field.IsNamed() {
				continue
			}
			out = append(out, identifier{
				text: field.Utf8Text(content),
				at:   int(field.StartByte()),
			})
		}
		return out
	}
	return []identifier{{text: named.Utf8Text(content), at: int(named.StartByte())}}
}

// read pulls one match apart into the kind it declares, the node
// declaring it, the node naming it, and the span it covers.
//
// It returns the two nodes rather than the name alone because a
// declaration can bind more names than one pattern can capture.
func read(
	match *ts.QueryMatch,
	names []string,
	p source.Path,
) (sema.Kind, *ts.Node, *ts.Node, source.Span, bool) {
	var (
		kind    sema.Kind
		node    *ts.Node
		named   *ts.Node
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
			// The first name is the one belonging to this declaration. A
			// later one comes from a nested declaration the same match
			// reached, as a union's body reaches its fields.
			if named == nil {
				named = &capture.Node
			}
			continue
		}
		if k, ok := KindOf(name); ok && !declare {
			kind, node, declare = k, &capture.Node, true
			span = spanOf(p, capture.Node)
		}
	}
	if !declare || named == nil {
		return 0, nil, nil, source.Span{}, false
	}
	return kind, node, named, span, true
}

// snippetOf returns the source text a span covers.
//
// Every symbol carries one and the output budget drops it for any detail
// level below full, which costs a copy per declaration on a scope that
// will not send it. The alternative is telling the engine what the
// caller intends to print, and how an answer is rendered is not
// something a parser should have to know.
//
// Offsets outside the content describe no text and yield none. They
// should not occur: the span came from a node in the tree this content
// was parsed into.
func snippetOf(content []byte, s source.Span) string {
	start, end := s.Start.Offset, s.End.Offset
	if start < 0 || end > len(content) || start >= end {
		return ""
	}
	return string(content[start:end])
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
