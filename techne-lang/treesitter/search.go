// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

// Search returns the declarations in a scope that match a query, best match
// first.
//
// A name matches in rank order: the exact text, the text with case folded,
// a prefix, then a substring. Within one rank a shorter name comes first,
// and the ID breaks the remaining ties, so identical requests return
// identical answers. An empty text matches every name. Without q.Private,
// Search leaves out the declarations that Outline reports as
// sema.Unexported. It returns the bindings that q.Include selects, and no
// other binding. Search reads the metadata of matching declarations only.
//
// q.Limit cuts the list after the filters. When it does, a caveat states how
// many of the matches the result contains.
func (e *Engine) Search(ctx context.Context, req engine.Request, q engine.Query) (engine.Result[sema.Symbol], error) {
	files, err := e.walk(req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	wanted := func(d named) bool {
		return (q.Kind == sema.KindUnknown || d.kind == q.Kind) &&
			(q.Private || d.visibility != sema.Unexported) &&
			q.Include.Keeps(d.kind, d.local) &&
			rank(d.name, q.Text) != noMatch
	}
	found, err := parse(ctx, e, files.Read, wanted, declaredIn)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}

	type ranked struct {
		symbol sema.Symbol
		rank   int
	}
	var matches []ranked
	for _, symbol := range slices.Concat(found...) {
		matches = append(matches, ranked{symbol: symbol, rank: rank(symbol.Name, q.Text)})
	}
	slices.SortStableFunc(matches, func(a, b ranked) int {
		return cmp.Or(
			cmp.Compare(a.rank, b.rank),
			cmp.Compare(len(a.symbol.Name), len(b.symbol.Name)),
			cmp.Compare(a.symbol.ID, b.symbol.ID),
		)
	})

	total := len(matches)
	if q.Limit > 0 && total > q.Limit {
		matches = matches[:q.Limit]
	}
	items := make([]sema.Symbol, len(matches))
	for i, m := range matches {
		items[i] = m.symbol
	}
	out := result(items, files, matchedText)
	if len(items) < total {
		out.Caveats = append(out.Caveats, trust.Caveat{
			Code: trust.CaveatTruncated,
			Note: fmt.Sprintf("%d of %d matches returned", len(items), total),
		})
	}
	return out, nil
}

// The ranks of a name match, best first.
const (
	literal = iota
	folded
	prefix
	substring
	noMatch
)

// rank returns how name matches wanted, and literal for every name when
// wanted is empty.
func rank(name, wanted string) int {
	if wanted == "" {
		return literal
	}
	lowered, lowWanted := strings.ToLower(name), strings.ToLower(wanted)
	switch {
	case name == wanted:
		return literal
	case lowered == lowWanted:
		return folded
	case strings.HasPrefix(lowered, lowWanted):
		return prefix
	case strings.Contains(lowered, lowWanted):
		return substring
	default:
		return noMatch
	}
}
