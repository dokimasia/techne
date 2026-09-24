// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"slices"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Plan returns the change of edit.DocumentSymbol, which writes documentation
// onto one declaration. That operation needs the position of the
// declaration and the comment forms of the language, and no name binding.
// Plan returns [engine.ErrDecline] for every other operation, because each
// one rewrites references or needs types.
//
// The documentation replaces the documentation the declaration has. Plan
// returns [engine.ErrRefuse] when the target is not a declaration, when the
// language cannot document its kind, and when the text contains the
// delimiter that closes the comment.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	if op != edit.DocumentSymbol {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: a parser cannot plan %s", engine.ErrDecline, op)
	}

	p, offset, err := e.site(ctx, req, target)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	content, err := e.contents(p)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	tree, grammar, err := e.parsed(p, content)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	defer tree.Close()

	node, kind, found := at(e.tags[grammar], tree, content, offset)
	if !found {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s declares nothing at byte %d", engine.ErrRefuse, p, offset)
	}
	one, err := documented(&node, content, e.declared.Comment, kind, args[edit.ArgDoc])
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("treesitter: %s: %w", p, err)
	}
	one.Span.Path = p

	return engine.Result[edit.Change]{
		Items:        []edit.Change{{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{one}}},
		Completeness: trust.ScopeTotal,
	}, nil
}

// site returns the file and the start offset of the declaration that target
// points at. For a span target, the span gives both. For a symbol target,
// site looks the ID up in the scope of req, and returns [engine.ErrDecline]
// when no declaration has the ID and [engine.ErrRefuse] when more than one
// has it. The refusal lists their positions, so the caller can point at one
// by span.
func (e *Engine) site(ctx context.Context, req engine.Request, target edit.Target) (source.Path, int, error) {
	if target.Kind == edit.TargetSpan {
		return target.Span.Path, target.Span.Start.Offset, nil
	}

	files, err := e.walk(req)
	if err != nil {
		return "", 0, err
	}
	name := target.Symbol.Name()
	per, err := parse(ctx, e, files.Read, func(d named) bool { return d.qualified == name }, declaredIn)
	if err != nil {
		return "", 0, err
	}
	found := slices.DeleteFunc(slices.Concat(per...), func(s sema.Symbol) bool { return s.ID != target.Symbol })

	switch len(found) {
	case 1:
		return found[0].Span.Path, found[0].Span.Start.Offset, nil
	case 0:
		if len(files.Unread) > 0 {
			return "", 0, fmt.Errorf("%w: %s was not looked up in %s, which is larger than %d bytes",
				engine.ErrDecline, target.Symbol, joined(files.Unread), lang.Largest)
		}
		return "", 0, fmt.Errorf("%w: %q declares no %s", engine.ErrDecline, req.Scope, target.Symbol)
	default:
		return "", 0, fmt.Errorf("%w: %q declares %s %d times, at %s",
			engine.ErrRefuse, req.Scope, target.Symbol, len(found), strings.Join(where(found), ", "))
	}
}

// joined returns paths separated by commas.
func joined(paths []source.Path) string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = string(p)
	}
	return strings.Join(out, ", ")
}

// where returns the path and one-based line of each declaration.
func where(found []sema.Symbol) []string {
	out := make([]string, len(found))
	for i, s := range found {
		out[i] = fmt.Sprintf("%s:%d", s.Span.Path, s.Span.Start.Line+1)
	}
	return out
}

// at returns the declaring node whose span starts at offset, with its kind.
// It runs the query of an outline and applies the same ranking, so the
// offsets an outline returns are the offsets at accepts.
func at(tags *ts.Query, tree *ts.Tree, content []byte, offset int) (ts.Node, sema.Kind, bool) {
	cursor := ts.NewQueryCursor()
	defer cursor.Close()

	names := tags.CaptureNames()
	matches := cursor.Matches(tags, tree.RootNode(), content)

	var (
		node  ts.Node
		kind  sema.Kind
		found bool
	)
	for match := matches.Next(); match != nil; match = matches.Next() {
		got, ok := read(match, names, "")
		if !ok || got.span.Start.Offset != offset {
			continue
		}
		if !found || MoreSpecific(got.kind, kind) {
			node, kind, found = *got.node, got.kind, true
		}
	}
	return node, kind, found
}

// documented returns the edit that writes text as the documentation of one
// declaration: above it, or as the first statement of its body when the
// language documents inside the declaration. It replaces the documentation
// the declaration has, because the documentation tools of these languages
// read one comment per declaration.
func documented(
	node *ts.Node,
	content []byte,
	style lang.CommentStyle,
	kind sema.Kind,
	text string,
) (edit.TextEdit, error) {
	if !documents(kind) {
		return edit.TextEdit{}, fmt.Errorf("%w: no language documents a %s on its own", engine.ErrRefuse, kind)
	}
	form := style.Documents()
	if form.Open == "" {
		return edit.TextEdit{}, fmt.Errorf("%w: the language declares no documentation form", engine.ErrRefuse)
	}
	if form.Close != "" && strings.Contains(text, form.Close) {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: the text contains %q, which closes the comment", engine.ErrRefuse, form.Close)
	}
	if form.Inside {
		return within(node, content, style, text)
	}
	return before(node, content, style, text)
}

// before returns the edit that writes documentation on the lines above a
// declaration and above the annotations written before it, as Java writes
// Javadoc above annotations and Rust writes /// above #[derive]. It
// replaces the documentation comments that [above] reads.
func before(node *ts.Node, content []byte, style lang.CommentStyle, text string) (edit.TextEdit, error) {
	anchor := outermost(node)
	for sibling := anchor.PrevNamedSibling(); sibling != nil; sibling = sibling.PrevNamedSibling() {
		if !annotationNodes[NodeKind(sibling.Kind())] {
			break
		}
		anchor = sibling
	}

	first, last := existing(anchor, content, style)
	region, end := int(anchor.StartByte()), 0
	if first != nil {
		region = int(first.StartByte())
		end = ended(content, int(last.EndByte()), region)
	}

	from, indent, ok := margin(content, region)
	if !ok {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: the declaration shares its line with other code, "+
				"so a comment above it documents that code", engine.ErrRefuse)
	}
	written := style.Document(text, indent)
	if first == nil {
		return edit.TextEdit{Span: span(from, from), New: written + "\n"}, nil
	}
	return edit.TextEdit{Span: span(from, end), New: written}, nil
}

// existing returns the first and last documentation comments above anchor,
// or nil. It stops at a blank line and at a comment in a form that is not
// documentation, as [above] does.
func existing(anchor *ts.Node, content []byte, style lang.CommentStyle) (first, last *ts.Node) {
	next := anchor
	for sibling := anchor.PrevNamedSibling(); sibling != nil; sibling = sibling.PrevNamedSibling() {
		if detached(sibling, next) {
			break
		}
		if _, isDoc := style.Documentation(sibling.Utf8Text(content)); !isDoc {
			break
		}
		if last == nil {
			last = sibling
		}
		first, next = sibling, sibling
	}
	return first, last
}

// within returns the edit that writes documentation as the first statement
// of the body of a declaration, as Python writes a docstring. It replaces a
// docstring the body has. A body written on the line of the declaration
// moves to its own line, indented by bodyIndent from the declaration.
func within(node *ts.Node, content []byte, style lang.CommentStyle, text string) (edit.TextEdit, error) {
	body := bodied(node)
	if body == nil {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: the language documents inside a declaration, and this declaration has no body",
			engine.ErrRefuse)
	}
	statement := body.NamedChild(0)
	if statement == nil {
		return edit.TextEdit{}, fmt.Errorf("%w: the body has no statement", engine.ErrRefuse)
	}

	if existing := docstring(statement); existing != nil {
		from, indent, ok := margin(content, int(existing.StartByte()))
		if !ok {
			return edit.TextEdit{}, fmt.Errorf(
				"%w: the documentation shares its line with the declaration", engine.ErrRefuse)
		}
		return edit.TextEdit{
			Span: span(from, ended(content, int(existing.EndByte()), from)),
			New:  style.Document(text, indent),
		}, nil
	}

	offset := int(statement.StartByte())
	from, indent, ownLine := margin(content, offset)
	if ownLine {
		return edit.TextEdit{Span: span(from, from), New: style.Document(text, indent) + "\n"}, nil
	}
	_, outer, _ := margin(content, int(node.StartByte()))
	indent = outer + bodyIndent
	return edit.TextEdit{
		Span: span(gap(content, offset), offset),
		New:  "\n" + style.Document(text, indent) + "\n" + indent,
	}, nil
}

// ended returns at moved back over the line terminators before it, but not
// before floor. Grammars differ on whether a comment node includes its
// newline, and the newline after a replaced comment stays in place.
func ended(content []byte, at, floor int) int {
	for at > floor && (content[at-1] == '\n' || content[at-1] == '\r') {
		at--
	}
	return at
}

// gap returns the offset where the spaces and tabs before at begin.
func gap(content []byte, at int) int {
	start := at
	for start > 0 && (content[start-1] == ' ' || content[start-1] == '\t') {
		start--
	}
	return start
}

// docstring returns statement when it is a string expression, which is a
// docstring as the first statement of a body, and nil otherwise.
func docstring(statement *ts.Node) *ts.Node {
	inner := statement
	if NodeKind(inner.Kind()) == NodeExpressionStatement {
		inner = inner.NamedChild(0)
	}
	if inner == nil || NodeKind(inner.Kind()) != NodeString {
		return nil
	}
	return statement
}

// margin returns the start of the line of at, the text between that start
// and at, and whether that text is whitespace only.
func margin(content []byte, at int) (start int, indent string, ownLine bool) {
	start = at
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	indent = string(content[start:at])
	return start, indent, strings.TrimLeft(indent, " \t") == ""
}

// span returns the range [from, to) without a path. The caller sets the
// path.
func span(from, to int) source.Span {
	return source.Span{Start: source.Position{Offset: from}, End: source.Position{Offset: to}}
}

// bodyIndent is the indentation a body receives when documentation moves
// it to its own line. Python, the language that documents inside a body,
// indents by four spaces.
const bodyIndent = "    "

var _ engine.Planner = (*Engine)(nil)
