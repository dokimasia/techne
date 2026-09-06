// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Plan computes the edits an operation would need.
//
// A parser serves one of them. Writing documentation onto a declaration
// needs the declaration's position, the language's comment forms and the
// text to write, and nothing about what any name means. Every other
// operation in the catalogue either rewrites the code that refers to its
// target or needs a type, so this declines them: a plan built on matched
// text would be wrong in exactly the cases nobody checks.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	if op != edit.DocumentSymbol {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: a parser cannot plan %s", engine.ErrDecline, op)
	}

	p, at, err := e.site(ctx, req, target)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}

	if unreadable := lang.Readable(e.fsys, p); unreadable != nil {
		return engine.Result[edit.Change]{}, unreadable
	}
	content, err := fs.ReadFile(e.fsys, string(p))
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("treesitter: read %s: %w", p, err)
	}

	held := e.grammar.For(string(p))
	parser := ts.NewParser()
	defer parser.Close()
	if unusable := parser.SetLanguage(held); unusable != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("treesitter: %s: %w", p, unusable)
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("treesitter: %s: parser returned no tree", p)
	}
	defer tree.Close()

	node, kind, found := e.at(e.tags[held], tree, content, at)
	if !found {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: %s declares nothing at byte %d", engine.ErrRefuse, p, at)
	}

	one, err := documented(node, content, e.declared.Comment, kind, args[edit.ArgDoc])
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("treesitter: %s: %w", p, err)
	}
	one.Span.Path = p

	return engine.Result[edit.Change]{
		Items:        []edit.Change{{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{one}}},
		Completeness: trust.ScopeTotal,
	}, nil
}

// site resolves what a target points at into a file and the byte the
// declaration starts at.
//
// A span says both and is believed. A symbol has to be looked for, and
// an identity is a language, a unit, a name and a kind, which a unit
// declaring two methods called Get satisfies twice. Two candidates are
// not one declaration, so they are reported rather than picked between.
func (e *Engine) site(
	ctx context.Context,
	req engine.Request,
	target edit.Target,
) (source.Path, int, error) {
	if target.Kind == edit.TargetSpan {
		return target.Span.Path, target.Span.Start.Offset, nil
	}

	declared, err := e.symbols(ctx, req)
	if err != nil {
		return "", 0, err
	}
	var found []sema.Symbol
	for _, s := range declared.items {
		if s.ID == target.Symbol {
			found = append(found, s)
		}
	}

	switch len(found) {
	case 1:
		return found[0].Span.Path, found[0].Span.Start.Offset, nil
	case 0:
		if len(declared.unread) > 0 {
			// "declares nothing" over a scope holding a file nothing
			// opened is the wrong answer to give: a caller acts on it by
			// believing the declaration is not there.
			return "", 0, fmt.Errorf(
				"%w: %s was not read, so %s was not looked for: past the %d bytes an engine parses",
				engine.ErrDecline, list(declared.unread), target.Symbol, lang.Largest)
		}
		return "", 0, fmt.Errorf("%w: %q declares no %s",
			engine.ErrDecline, req.Scope, target.Symbol)
	default:
		return "", 0, fmt.Errorf(
			"%w: %q declares %s %d times, so it names no one declaration: %s",
			engine.ErrRefuse, req.Scope, target.Symbol, len(found), strings.Join(where(found), ", "))
	}
}

// list names the files a scope holds that were not read.
func list(paths []source.Path) string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, string(p))
	}
	return strings.Join(out, ", ")
}

// where lists the positions several declarations sharing one identity
// were found at, so a caller can point at one of them by span.
func where(found []sema.Symbol) []string {
	out := make([]string, 0, len(found))
	for _, s := range found {
		out = append(out, fmt.Sprintf("%s:%d", s.Span.Path, s.Span.Start.Line+1))
	}
	return out
}

// at finds the node a declaration was reported from, by the byte it
// starts at.
//
// It runs the same query the outline runs and reads each match the same
// way, so the offsets an outline hands back are the offsets this
// answers to. A byte that several patterns match is resolved as the
// outline resolves it, by rank.
func (*Engine) at(
	tags *ts.Query,
	tree *ts.Tree,
	content []byte,
	offset int,
) (*ts.Node, sema.Kind, bool) {
	cursor := ts.NewQueryCursor()
	defer cursor.Close()

	names := tags.CaptureNames()
	matches := cursor.Matches(tags, tree.RootNode(), content)

	var (
		node  *ts.Node
		found sema.Kind
	)
	for match := matches.Next(); match != nil; match = matches.Next() {
		kind, declaring, _, span, ok := read(match, names, "")
		if !ok || span.Start.Offset != offset {
			continue
		}
		if node == nil || Outranks(kind, found) {
			node, found = declaring, kind
		}
	}
	return node, found, node != nil
}

// documented returns the edit that writes documentation onto one
// declaration.
//
// Two shapes, and the language says which: a comment written above the
// declaration, or a string written as the first statement of its body.
// Either way documentation already there is replaced rather than added
// to, because a declaration carrying two doc comments is one the
// language's own tool reads only half of.
func documented(
	node *ts.Node,
	content []byte,
	style lang.CommentStyle,
	kind sema.Kind,
	text string,
) (edit.TextEdit, error) {
	if !documents(kind) {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: no language documents a %s as a declaration of its own", engine.ErrRefuse, kind)
	}
	form := style.Documents()
	if form.Open == "" {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: this language declares no way to write documentation", engine.ErrRefuse)
	}
	// A delimiter inside the text ends the comment where the text meant
	// to continue, and what follows is code. The gate would catch it as
	// a file that stopped parsing, which is a true report of the wrong
	// problem.
	if form.Close != "" && strings.Contains(text, form.Close) {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: the text holds %q, which closes the comment it would be written in",
			engine.ErrRefuse, form.Close)
	}

	if form.Inside {
		return within(node, content, style, text)
	}
	return before(node, content, style, text)
}

// before writes documentation on the lines before a declaration.
//
// The comment goes above whatever is written above the declaration:
// Java puts Javadoc before the annotations, Rust puts /// before
// #[derive], and both grammars make those siblings. What is already
// documentation there is what this replaces.
func before(
	node *ts.Node,
	content []byte,
	style lang.CommentStyle,
	text string,
) (edit.TextEdit, error) {
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
			"%w: the declaration shares its line with the code before it, "+
				"so a comment above it would document that", engine.ErrRefuse)
	}

	written := style.Document(text, indent)
	if first == nil {
		// Nothing to replace, so the comment is a line inserted at the
		// margin and everything below it keeps the indentation already
		// written before it.
		return edit.TextEdit{Span: span(from, from), New: written + "\n"}, nil
	}
	return edit.TextEdit{Span: span(from, end), New: written}, nil
}

// existing returns the first and last comment nodes documenting a
// declaration, or nil.
//
// It walks back by the rule [above] reads by, so what is replaced is
// exactly what would have been reported: consecutive comments in one of
// the language's documentation forms, ending at a blank line or at
// anything that is not documentation.
func existing(
	anchor *ts.Node,
	content []byte,
	style lang.CommentStyle,
) (first, last *ts.Node) {
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

// within writes documentation as the first statement of a declaration's
// body, which is how Python documents.
func within(
	node *ts.Node,
	content []byte,
	style lang.CommentStyle,
	text string,
) (edit.TextEdit, error) {
	body := bodied(node)
	if body == nil {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: this language writes documentation inside what it documents, "+
				"and this declaration has no body", engine.ErrRefuse)
	}
	statement := body.NamedChild(0)
	if statement == nil {
		return edit.TextEdit{}, fmt.Errorf(
			"%w: the body holds no statement to write documentation before", engine.ErrRefuse)
	}

	if existing := docstring(statement); existing != nil {
		from, indent, ok := margin(content, int(existing.StartByte()))
		if !ok {
			return edit.TextEdit{}, fmt.Errorf(
				"%w: the documentation shares its line with the declaration, "+
					"so replacing it would join the two", engine.ErrRefuse)
		}
		return edit.TextEdit{
			Span: span(from, ended(content, int(existing.EndByte()), from)),
			New:  style.Document(text, indent),
		}, nil
	}

	at := int(statement.StartByte())
	from, indent, ownLine := margin(content, at)
	if ownLine {
		// The body is already laid out, so the documentation is a line
		// inserted at the margin and the statement keeps the indentation
		// already written before it.
		return edit.TextEdit{
			Span: span(from, from),
			New:  style.Document(text, indent) + "\n",
		}, nil
	}

	// The body is written on the declaration's line. Documentation
	// cannot go there, so the body moves down one level from whatever
	// the declaration itself is indented to. What is replaced is the gap
	// after the declaration and not the line it is on: the line holds
	// the declaration.
	_, outer, _ := margin(content, int(node.StartByte()))
	indent = outer + bodyIndent
	return edit.TextEdit{
		Span: span(gap(content, at), at),
		New:  "\n" + style.Document(text, indent) + "\n" + indent,
	}, nil
}

// ended returns where a replaced range stops, without the line ending.
//
// Whether a comment node holds the newline that terminates it is the
// grammar's decision and they disagree: tree-sitter-rust puts it inside
// the node and tree-sitter-go does not. What is being replaced is the
// text of a comment, so the line ending after it belongs to the
// declaration below and stays where it is.
func ended(content []byte, at, floor int) int {
	for at > floor && (content[at-1] == '\n' || content[at-1] == '\r') {
		at--
	}
	return at
}

// gap returns where the run of spaces and tabs before an offset begins.
//
// It is what separates two things written on one line, as against
// [margin], which is the whitespace a line begins with.
func gap(content []byte, at int) int {
	start := at
	for start > 0 && (content[start-1] == ' ' || content[start-1] == '\t') {
		start--
	}
	return start
}

// docstring returns the string a statement writes as documentation, or
// nil. A grammar wraps it in an expression statement, which is what
// would be replaced but not what is looked for.
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

// margin returns the start of the line an offset sits on, the whitespace
// written before it, and whether that is all there is.
//
// A declaration sharing its line with code has nowhere to put a comment
// that documents it and nothing else, which the caller refuses rather
// than guesses at.
func margin(content []byte, at int) (start int, indent string, ownLine bool) {
	start = at
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	indent = string(content[start:at])
	return start, indent, strings.TrimLeft(indent, " \t") == ""
}

// span is a range within one file. The path is filled in by the caller,
// which is the one that knows it.
func span(from, to int) source.Span {
	return source.Span{
		Start: source.Position{Offset: from},
		End:   source.Position{Offset: to},
	}
}

// bodyIndent is what a body is moved in by when it has to be moved onto
// its own line. Only a language documenting inside a body reaches this,
// and the one that does spells its indentation four spaces.
const bodyIndent = "    "

// assert the engine serves the role it claims.
var _ engine.Planner = (*Engine)(nil)
