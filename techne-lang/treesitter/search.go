// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
)

// Search reports the declarations in a scope matching a query, best
// match first.
//
// An engine returns its own best order and nothing re-ranks it. A
// language server ranks with more to go on than a parser has; overriding
// that would throw away what it knows.
//
// Ranking here is by how the name matched. A literal match beats one
// that needed folding, because a language where store and Store are two
// declarations means the caller who typed one wanted that one. Below
// those come prefix and substring, and a shorter name sorts before a
// longer one at the same rank so Get comes above GetOrCreate. Identity
// breaks any remaining tie, so two identical requests answer
// identically.
func (e *Engine) Search(ctx context.Context, req engine.Request, q engine.Query) (engine.Result[sema.Symbol], error) {
	all, read, err := e.symbols(ctx, req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}

	matched := make([]sema.Symbol, 0, len(all))
	for _, s := range all {
		if q.Kind != sema.KindUnknown && s.Kind != q.Kind {
			continue
		}
		if !q.Private && s.Visibility == sema.Unexported {
			continue
		}
		if q.Text != "" && rank(s.Name, q.Text) == noMatch {
			continue
		}
		matched = append(matched, s)
	}

	slices.SortStableFunc(matched, func(a, b sema.Symbol) int {
		if byRank := cmp.Compare(rank(a.Name, q.Text), rank(b.Name, q.Text)); byRank != 0 {
			return byRank
		}
		if byLength := cmp.Compare(len(a.Name), len(b.Name)); byLength != 0 {
			return byLength
		}
		return cmp.Compare(a.ID, b.ID)
	})

	if q.Limit > 0 && len(matched) > q.Limit {
		matched = matched[:q.Limit]
	}
	return found(matched, read), nil
}

// How a name matched, lowest first. noMatch sorts last and is filtered
// out before ranking ever sees it.
const (
	literal = iota
	folded
	prefix
	substring
	noMatch
)

// rank reports how a name matched the wanted text. An empty query
// matches everything equally.
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
