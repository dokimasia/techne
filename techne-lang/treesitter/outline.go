// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"io/fs"
	"runtime"
	"slices"
	"sync"
	"time"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// matchedText is the note of the caveat of every outline and search.
const matchedText = "a parser matched text, so a name that crosses a file is matched by spelling"

// Outline returns the declarations in the files of a scope.
//
// A directory scope includes every file of the language under it, and a
// file scope of another language returns a result with Skipped set. The
// coverage is total, except that a file larger than [lang.Largest] is not
// parsed: a caveat names it and the coverage is partial.
func (e *Engine) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	files, err := e.walk(req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	found, err := parse(ctx, e, files.Read, nil, declaredIn)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	return result(slices.Concat(found...), files, matchedText), nil
}

// walk returns the files of the scope of req, without the files the
// language treats as tests unless req includes them.
func (e *Engine) walk(req engine.Request) (lang.Files, error) {
	files, err := lang.Walk(e.fsys, req.Scope, e.declared.Extensions)
	if err != nil || req.Tests {
		return files, err
	}
	test := func(p source.Path) bool { return e.declared.IsTest(string(p)) }
	files.Read = slices.DeleteFunc(files.Read, test)
	files.Unread = slices.DeleteFunc(files.Unread, test)
	return files, nil
}

// keep selects the declarations whose metadata a caller reads. A nil keep
// selects every declaration.
type keep func(d named) bool

// named is what a keep reads of one declaration: its name, its qualified
// name, its kind, its visibility, and whether it is local as [sema.Locals]
// reports.
type named struct {
	name, qualified string
	kind            sema.Kind
	visibility      sema.Visibility
	local           bool
}

// scan is the record of one parse of a file: the size and the modification
// time of the file, and each distinct declaration the parse matched.
type scan struct {
	size     int64
	modified time.Time
	declared []named
}

// selects reports whether selected keeps a declaration of s.
func (s scan) selects(selected keep) bool {
	return slices.ContainsFunc(s.declared, selected)
}

// scans keeps the latest scan of each file that the engine parsed, until the
// engine closes. It is safe for concurrent use.
type scans struct {
	mu    sync.Mutex
	files map[source.Path]scan
}

// fresh returns the scan of the file at p, and reports whether the engine
// scanned the file at the size and the modification time of info.
func (s *scans) fresh(p source.Path, info fs.FileInfo) (scan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept, known := s.files[p]
	return kept, known && kept.size == info.Size() && kept.modified.Equal(info.ModTime())
}

// record keeps the declarations of the file at p, parsed at the size and the
// modification time of info.
func (s *scans) record(p source.Path, info fs.FileInfo, declared []named) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[p] = scan{size: info.Size(), modified: info.ModTime(), declared: declared}
}

// parse reads and parses files in parallel, at most one file per CPU, and
// returns what visit returns for each file, in the order of files. visit
// runs on a worker goroutine. parse returns the first error in the order of
// files, and ctx.Err() when ctx is done before every file started.
//
// For a selected keep, parse does not read a file that the engine scanned at
// its current size and modification time when selected keeps none of its
// declarations. visit then receives nil content and no declarations.
func parse[T any](
	ctx context.Context,
	e *Engine,
	files []source.Path,
	selected keep,
	visit func(p source.Path, content []byte, declared []sema.Symbol) T,
) ([]T, error) {
	out := make([]T, len(files))
	failed := make([]error, len(files))
	room := make(chan struct{}, max(runtime.NumCPU(), 1))
	var wait sync.WaitGroup
	for at, p := range files {
		if err := ctx.Err(); err != nil {
			wait.Wait()
			return nil, err
		}
		room <- struct{}{}
		wait.Go(func() {
			defer func() { <-room }()
			info, unstated := fs.Stat(e.fsys, string(p))
			if unstated == nil && selected != nil {
				if kept, fresh := e.scans.fresh(p, info); fresh && !kept.selects(selected) {
					out[at] = visit(p, nil, nil)
					return
				}
			}
			content, err := fs.ReadFile(e.fsys, string(p))
			if err != nil {
				failed[at] = fmt.Errorf("treesitter: read %s: %w", p, err)
				return
			}
			declared, all, err := e.declarations(p, content, selected)
			if err != nil {
				failed[at] = err
				return
			}
			if unstated == nil {
				e.scans.record(p, info, all)
			}
			out[at] = visit(p, content, declared)
		})
	}
	wait.Wait()
	for _, err := range failed {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// declaredIn returns the declarations of one file. It is the visit function
// of a caller that wants the declarations themselves.
func declaredIn(_ source.Path, _ []byte, declared []sema.Symbol) []sema.Symbol { return declared }

// result returns items with the evidence of a walk: Skipped when the scope
// contains no file of the language, a CaveatDynamic caveat with note, and a
// CaveatUnread caveat with partial coverage when the walk found files larger
// than [lang.Largest].
func result[T any](items []T, files lang.Files, note string) engine.Result[T] {
	caveats := []trust.Caveat{{Code: trust.CaveatDynamic, Note: note}}
	covered := trust.ScopeTotal
	if len(files.Unread) > 0 {
		covered = trust.ScopePartial
		caveats = append(caveats, trust.Caveat{
			Code:  trust.CaveatUnread,
			Note:  fmt.Sprintf("larger than %d bytes, so not parsed", lang.Largest),
			Paths: files.Unread,
		})
	}
	return engine.Result[T]{
		Items:        items,
		Skipped:      len(files.Read) == 0 && len(files.Unread) == 0,
		Completeness: covered,
		Caveats:      caveats,
	}
}

// declaration is one declaration the tags query matched, before its
// metadata is read.
type declaration struct {
	kind sema.Kind
	// node is the declaring node. It is valid while the tree is open.
	node ts.Node
	name string
	// receiver is the name that a [Receiver] capture gives the type of the
	// declaration, or empty.
	receiver string
	// start is the first byte of the declaring node. Declarations with one
	// start come from one statement that binds more than one name.
	start uint
	span  source.Span
}

// qualify returns the qualified name of each declaration of found, whose
// containers are the indexes that [sema.Containers] returns. A declaration
// is qualified by its receiver when it has one, and by the qualified name of
// its container otherwise. An import is not qualified, because its name is a
// path.
func qualify(found []declaration, containers []int) []string {
	out := make([]string, len(found))
	done := make([]bool, len(found))
	var of func(i int) string
	of = func(i int) string {
		if done[i] {
			return out[i]
		}
		d, container := found[i], ""
		switch {
		case d.kind == sema.KindImport:
		case d.receiver != "":
			container = d.receiver
		case containers[i] >= 0:
			container = of(containers[i])
		}
		out[i], done[i] = sema.Qualify(container, d.name), true
		return out[i]
	}
	for i := range found {
		of(i)
	}
	return out
}

// declarations parses one file and returns its declarations, in the order
// the query matched them, and each distinct declaration of the file as a keep
// reads it.
//
// It matches every declaration, links each to its container, qualifies its
// name and computes its visibility first. It then reads the metadata of the
// declarations that selected keeps, so a caller that returns few
// declarations reads the metadata of few.
func (e *Engine) declarations(p source.Path, content []byte, selected keep) ([]sema.Symbol, []named, error) {
	tree, grammar, err := e.parsed(p, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	unit := source.Path(e.declared.Namespace(string(p)))
	found := e.matched(tree, content, grammar, p)

	linked := make([]sema.Symbol, len(found))
	for i, d := range found {
		linked[i] = sema.Symbol{Kind: d.kind, Span: d.span}
	}
	containers := sema.Containers(linked)
	locals := sema.Locals(linked, containers)
	qualified := qualify(found, containers)
	statements := map[uint]int{}
	for i, d := range found {
		linked[i].ID = sema.NewID(e.declared.Language, unit, qualified[i], d.kind)
		statements[d.start]++
	}
	link(linked, containers)

	var out []sema.Symbol
	var all []named
	distinct := map[named]bool{}
	for i, d := range found {
		one := named{
			name:       d.name,
			qualified:  qualified[i],
			kind:       d.kind,
			visibility: e.visibility(found, containers, i),
			local:      locals[i],
		}
		if !distinct[one] {
			distinct[one] = true
			all = append(all, one)
		}
		if selected != nil && !selected(one) {
			continue
		}
		visibility := one.visibility
		symbol := e.symbol(d, content, p, linked[i].ID)
		symbol.Parent = linked[i].Parent
		symbol.Visibility = visibility
		if statements[d.start] > 1 {
			// A statement that binds more than one name, such as
			// `const a, b = 1, 2`, is the signature of none of them.
			symbol.Signature = symbol.Name
		}
		out = append(out, symbol)
	}
	return out, all, nil
}

// parsed returns the tree of content, parsed with the grammar of the file
// at p, and that grammar. The caller closes the tree.
func (e *Engine) parsed(p source.Path, content []byte) (*ts.Tree, *ts.Language, error) {
	grammar := e.grammar.For(string(p))
	parser := ts.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(grammar); err != nil {
		return nil, nil, fmt.Errorf("treesitter: %s: %w", p, err)
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, nil, fmt.Errorf("treesitter: %s: parser returned no tree", p)
	}
	return tree, grammar, nil
}

// contents returns the content of the file at p, and the error of
// [lang.Readable] for a file that the workspace excludes or that is larger
// than [lang.Largest].
func (e *Engine) contents(p source.Path) ([]byte, error) {
	if err := lang.Readable(e.fsys, p); err != nil {
		return nil, err
	}
	content, err := fs.ReadFile(e.fsys, string(p))
	if err != nil {
		return nil, fmt.Errorf("treesitter: read %s: %w", p, err)
	}
	return content, nil
}

// matched runs the tags query over tree and returns one declaration per
// declared name, with the kind of the highest-ranked pattern that matched
// it, and the receiver of any pattern that captured one.
func (e *Engine) matched(tree *ts.Tree, content []byte, grammar *ts.Language, p source.Path) []declaration {
	query := e.tags[grammar]
	names := query.CaptureNames()
	cursor := ts.NewQueryCursor()
	defer cursor.Close()
	walk := tree.RootNode().Walk()
	defer walk.Close()

	var out []declaration
	// index maps the offset of a declared name to its entry in out. Two
	// patterns that match one declaration name the same identifier.
	index := map[int]int{}
	matches := cursor.Matches(query, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		got, ok := read(match, names, p)
		if !ok {
			continue
		}
		receiver := ""
		if got.receiver != nil {
			receiver = got.receiver.Utf8Text(content)
		}
		for _, one := range bound(got.node, got.named, walk, content) {
			name := unquote(one.text)
			if e.declared.Blank[name] {
				continue
			}
			if i, seen := index[one.at]; seen {
				if MoreSpecific(got.kind, out[i].kind) {
					out[i].kind = got.kind
				}
				if out[i].receiver == "" {
					out[i].receiver = receiver
				}
				continue
			}
			index[one.at] = len(out)
			out = append(out, declaration{
				kind: got.kind, node: *got.node, name: name, receiver: receiver,
				start: got.node.StartByte(), span: got.span,
			})
		}
	}
	return out
}

// symbol reads the metadata of one declaration from the tree. The caller
// sets the parent and the visibility, which depend on the other
// declarations of the file.
func (e *Engine) symbol(d declaration, content []byte, p source.Path, id sema.ID) sema.Symbol {
	marks := annotations(&d.node, content, p)
	signed := signature(&d.node, content, marks, d.kind, e.declared.Comment)
	if d.kind == sema.KindField {
		marks = append(marks, tags(&d.node, content, p)...)
	}
	return sema.Symbol{
		ID:          id,
		Name:        d.name,
		Kind:        d.kind,
		Language:    e.declared.Language,
		Span:        d.span,
		Modifiers:   modifiers(&d.node, content),
		Annotations: marks,
		Signature:   signed,
		Doc:         documentation(&d.node, content, e.declared.Comment, d.kind),
		Snippet:     snippetOf(content, d.span),
	}
}

// unquote removes the quotes around a name that a grammar gives as a string
// literal, as most languages write the target of an import.
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

// identifier is one name a declaration binds, and the offset of the name.
type identifier struct {
	text string
	at   int
}

// bound returns every name one declaration binds.
//
// A declaring node with more than one name field binds each of them, as in
// Go's `const a, b = 1, 2`. A capture matches one node, so bound reads those
// names from the fields. Otherwise the name capture is the name, as for
// Python's assignment, which writes the name under left, and for Go's
// embedded field, which has no name field.
func bound(node, named *ts.Node, walk *ts.TreeCursor, content []byte) []identifier {
	if fields := node.ChildrenByFieldName(string(FieldNameName), walk); len(fields) > 1 {
		out := make([]identifier, 0, len(fields))
		for _, field := range fields {
			// ChildrenByFieldName also returns the separators between the
			// fields, which are not named nodes.
			if !field.IsNamed() {
				continue
			}
			out = append(out, identifier{text: field.Utf8Text(content), at: int(field.StartByte())})
		}
		return out
	}
	return []identifier{{text: named.Utf8Text(content), at: int(named.StartByte())}}
}

// captured is what one match of the tags query captures of a declaration.
type captured struct {
	kind sema.Kind
	// node is the declaring node, and named the node of its name.
	node, named *ts.Node
	// receiver is the node of the [Receiver] capture, or nil.
	receiver *ts.Node
	span     source.Span
}

// read returns what one match captures of a declaration, and reports false
// for a match without a definition capture or without a name capture. The
// first name capture belongs to the declaration. A later one belongs to a
// declaration nested in it, as a field in the body of a union.
func read(match *ts.QueryMatch, names []string, p source.Path) (captured, bool) {
	var out captured
	defines := false
	for _, capture := range match.Captures {
		index := int(capture.Index)
		if index < 0 || index >= len(names) {
			continue
		}
		switch name := Capture(names[index]); name {
		case Name:
			if out.named == nil {
				out.named = &capture.Node
			}
		case Receiver:
			if out.receiver == nil {
				out.receiver = &capture.Node
			}
		default:
			if k, ok := KindOf(name); ok && !defines {
				out.kind, out.node, defines = k, &capture.Node, true
				out.span = spanOf(p, capture.Node)
			}
		}
	}
	return out, defines && out.named != nil
}

// snippetOf returns the text of content that s covers, or an empty string
// for a span outside content.
func snippetOf(content []byte, s source.Span) string {
	start, end := s.Start.Offset, s.End.Offset
	if start < 0 || end > len(content) || start >= end {
		return ""
	}
	return string(content[start:end])
}

// spanOf returns the span of n in the file at p. tree-sitter counts rows and
// byte columns, as source.Position does.
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

var _ interface {
	engine.Engine
	engine.Outliner
} = (*Engine)(nil)
