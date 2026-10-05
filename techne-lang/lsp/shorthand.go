// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Shorthands reads the shorthand properties of a file: the names in the object literals of the
// file that are the key and the value of a property at once, as file in { file }. The outline
// engine of JavaScript and of TypeScript implements it.
type Shorthands interface {
	Shorthands(ctx context.Context, p source.Path) ([]source.Span, error)
}

// shorthanded writes out each edit of changes that replaces the name of a shorthand property
// with a bare new name, and returns the caveat about the first such edit whose sides it cannot
// tell apart, or nil. typescript-language-server and tsc --lsp write the bare new name, because
// [TypeScriptSettings] and [NativeSettings] turn off the text that the servers write around a
// renamed name. The shorthand property { file } has the key file and the value file, and its
// edit writes:
//
//   - file: blob, when changes rewrite a declaration of the value and none of the key
//   - blob: file, when they rewrite a declaration of the key and none of the value
//   - blob, when they rewrite both, as a server does when the key is also bound by a
//     destructuring pattern
//
// [spelling.sides] reads the two sides from the definition at the name. An edit whose sides it
// cannot read keeps its bare name, and the first one gets the caveat. For an engine whose outline
// engine does not implement [Shorthands], shorthanded leaves changes as they are.
func (e *Engine) shorthanded(ctx context.Context, held *session, changes []edit.Change) (*trust.Caveat, error) {
	reads, implements := e.outliner.(Shorthands)
	if !implements {
		return nil, nil
	}
	s := spelling{
		engine: e, session: held, found: newFinder(e, held), reads: reads,
		names: map[source.Path][]source.Span{}, edits: map[source.Path][]edit.TextEdit{},
	}
	for _, c := range changes {
		if c.Kind == edit.ChangeEdit {
			s.edits[c.Path] = append(s.edits[c.Path], c.Edits...)
		}
	}
	var first *trust.Caveat
	for i := range changes {
		c := &changes[i]
		if c.Kind != edit.ChangeEdit {
			continue
		}
		names, err := s.shorthands(ctx, c.Path)
		if err != nil {
			return nil, err
		}
		for j := range c.Edits {
			one := &c.Edits[j]
			if !slices.Contains(names, extent(one.Span)) {
				continue
			}
			written, known, err := s.written(ctx, c.Path, *one)
			switch {
			case err != nil:
				return nil, err
			case known:
				one.New = written
			case first == nil:
				first = &trust.Caveat{
					Code: trust.CaveatUnrewritten,
					Note: fmt.Sprintf("the rename writes %s for the shorthand property at %s:%d, and the "+
						"definition there does not show whether it renames the key or the value",
						one.New, c.Path, one.Span.Start.Line+1),
				}
			}
		}
	}
	return first, nil
}

// spelling is the state of one call of [Engine.shorthanded]: the shorthand properties of each
// file that it read, and the edits of the plan by file.
type spelling struct {
	engine  *Engine
	session *session
	found   *finder
	reads   Shorthands
	names   map[source.Path][]source.Span
	edits   map[source.Path][]edit.TextEdit
}

// shorthands returns the spans of the shorthand properties of the file at p, without their
// paths, and reads each file once. A file that [lang.Readable] refuses has none.
func (s spelling) shorthands(ctx context.Context, p source.Path) ([]source.Span, error) {
	if kept, read := s.names[p]; read {
		return kept, nil
	}
	found, err := s.reads.Shorthands(ctx, p)
	if refused(err) {
		found, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]source.Span, 0, len(found))
	for _, one := range found {
		out = append(out, extent(one))
	}
	s.names[p] = out
	return out, nil
}

// written returns the text of the edit one at a shorthand property of the file at p, and
// reports whether [spelling.sides] tells which sides the plan renames. An edit that writes text
// with a colon is returned as it is, because the server wrote the property out itself.
func (s spelling) written(ctx context.Context, p source.Path, one edit.TextEdit) (string, bool, error) {
	if strings.Contains(one.New, ":") {
		return one.New, true, nil
	}
	kept, err := s.found.file(ctx, p)
	if err != nil {
		return "", false, err
	}
	old := kept.doc.text(one.Span)
	value, key, err := s.sides(ctx, kept.doc, one.Span)
	switch {
	case err != nil:
		return "", false, err
	case value && key:
		return one.New, true, nil
	case value:
		return old + ": " + one.New, true, nil
	case key:
		return one.New + ": " + old, true, nil
	}
	return "", false, nil
}

// sides reports whether the plan rewrites a declaration of the value and one of the key of the
// shorthand property whose name is the span name of doc. The definition at the name returns the
// declarations of both. The declaration of the outline whose name is at a location is the key
// when [keyed] reports its kind, and the value otherwise. These locations count for neither
// side:
//
//   - a declaration that the plan does not rewrite
//   - the name of a shorthand property, whose key [Engine.shorthanded] writes out
//   - a location at which the outline does not declare a name
//
// For a server that does not serve textDocument/definition, sides reports false for both.
func (s spelling) sides(ctx context.Context, doc document, name source.Span) (value, key bool, err error) {
	e := s.engine
	if !provides(s.session.capable.DefinitionProvider) {
		return false, false, nil
	}
	if _, err = e.open(ctx, s.session, doc.path); err != nil {
		return false, false, err
	}
	declared, err := e.definedAt(ctx, s.session, doc, doc.mark(name.Start))
	if err != nil {
		return false, false, err
	}
	for _, one := range declared {
		p := e.pathOf(one.URI)
		if lang.Outside(p) {
			continue
		}
		kept, err := s.found.file(ctx, p)
		if err != nil {
			return false, false, err
		}
		names, err := s.shorthands(ctx, p)
		if err != nil {
			return false, false, err
		}
		where := kept.doc.span(one.Range)
		if slices.Contains(names, extent(where)) || !rewrites(s.edits[p], where) {
			continue
		}
		at := slices.IndexFunc(kept.symbols, func(decl sema.Symbol) bool {
			return naming(kept.doc, decl) == one.Range.Start
		})
		switch {
		case at < 0:
		case keyed(kept.symbols[at].Kind):
			key = true
		default:
			value = true
		}
	}
	return value, key, nil
}

// keyed reports whether a declaration of kind is the key of a property: a field or a method,
// the kinds that the outline engine of JavaScript and of TypeScript gives the members of a type
// and the keys of an object literal.
func keyed(kind sema.Kind) bool {
	return kind == sema.KindField || kind == sema.KindMethod
}

// extent returns the offsets of s without its path and its lines and columns, which tell whether
// two spans of one file cover the same bytes.
func extent(s source.Span) source.Span {
	return source.Span{Start: source.Position{Offset: s.Start.Offset}, End: source.Position{Offset: s.End.Offset}}
}
