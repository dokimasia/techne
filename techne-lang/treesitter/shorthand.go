// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// Shorthands returns the span of each capture of the shorthands query of the grammar in the file
// at p, in the order of the file: the name of each shorthand property of an object literal, as
// file in { file }. A grammar without the query returns no span, and so does a file of another
// language.
//
// Shorthands returns the error of [lang.Readable] for a file that the workspace excludes or that
// is larger than [lang.Largest].
func (e *Engine) Shorthands(_ context.Context, p source.Path) ([]source.Span, error) {
	if len(e.shorthands) == 0 || !lang.Claims(string(p), e.declared.Extensions) {
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

	cursor := ts.NewQueryCursor()
	defer cursor.Close()
	var out []source.Span
	matches := cursor.Matches(e.shorthands[grammar], tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			out = append(out, spanOf(p, capture.Node))
		}
	}
	return out, nil
}
