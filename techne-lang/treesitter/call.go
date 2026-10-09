// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"cmp"
	"context"
	"slices"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// Calls returns the span of the name of each callee that a call of the file at p names, in the
// order of the file and each span once. A call is a match of the tags query with a
// [ReferenceCall] or a [ReferenceSend] capture, and its [Name] capture is the name. A name
// that the call qualifies, as the Get of `store.Get()`, is the name of the member. A name that
// a definition capture names too is a declaration and not a call: the tags query of Ruby
// captures every identifier that is not a local as a call, the name of a method included.
//
// Calls returns no span for a file of another language, and the error of [lang.Readable] for a
// file that the workspace excludes or that is larger than [lang.Largest].
func (e *Engine) Calls(_ context.Context, p source.Path) ([]source.Span, error) {
	if !lang.Claims(string(p), e.declared.Extensions) {
		return nil, nil
	}
	content, err := e.contents(p)
	if err != nil {
		return nil, err
	}
	tree, grammar, err := e.parsed(p, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	query := e.tags[grammar]
	names := query.CaptureNames()
	cursor := ts.NewQueryCursor()
	defer cursor.Close()
	var out []source.Span
	declares := map[int]bool{}
	matches := cursor.Matches(query, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		var name *ts.Node
		calls, defines := false, false
		for _, capture := range match.Captures {
			switch c := Capture(names[capture.Index]); c {
			case Name:
				if name == nil {
					name = &capture.Node
				}
			case ReferenceCall, ReferenceSend:
				calls = true
			default:
				_, kinded := KindOf(c)
				defines = defines || kinded
			}
		}
		switch {
		case name == nil:
		case defines:
			declares[int(name.StartByte())] = true
		case calls:
			out = append(out, spanOf(p, *name))
		}
	}
	out = slices.DeleteFunc(out, func(one source.Span) bool { return declares[one.Start.Offset] })
	slices.SortFunc(out, func(a, b source.Span) int {
		return cmp.Or(cmp.Compare(a.Start.Offset, b.Start.Offset), cmp.Compare(a.End.Offset, b.End.Offset))
	})
	return slices.Compact(out), nil
}
