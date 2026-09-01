// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"encoding/json"
	"fmt"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Detail is how much of each item an answer carries.
//
// The zero value is [Standard].
type Detail string

const (
	// Standard identifies a declaration and says where it is. It is what
	// an agent that names no level receives.
	Standard Detail = ""
	// Summary identifies a declaration and no more.
	Summary Detail = "summary"
	// Full adds the documentation comment and the declaration's own
	// source text.
	Full Detail = "full"
)

// DefaultMaxTokens is the budget an agent that names none receives.
//
// Nobody has measured it against a real agent on a real repository. The
// answer reports what it spent, so real calls correct this rather than
// anyone arguing about it.
const DefaultMaxTokens = 6000

// bytesPerToken is the estimate. Code and identifiers run denser than
// prose, and the figure is deliberately low so the estimate over-counts
// and the budget is not exceeded.
const bytesPerToken = 3

// Budget is how much of an answer a caller will take.
type Budget struct {
	// MaxTokens is the estimated ceiling. Zero means
	// [DefaultMaxTokens].
	MaxTokens int
	// Detail selects what each item carries before the ceiling applies.
	Detail Detail
}

// Fit returns the answer thinned to the budget.
//
// It applies [Budget.Detail] first, then removes documentation from
// every item, then the source text, then drops items from the end. An
// answer that lost items carries [trust.CaveatTruncated] naming how many
// matched.
//
// Prose goes before code because a caller that asked what a scope
// declares can act on a name alone, and the sentence describing it was
// the least of what it asked for.
//
// The provenance is never changed: thinning an answer says nothing about
// the evidence behind it.
func Fit(a engine.Answer[sema.Symbol], b Budget) engine.Answer[sema.Symbol] {
	ceiling := b.MaxTokens
	if ceiling <= 0 {
		ceiling = DefaultMaxTokens
	}

	matched := len(a.Items)
	a.Items = project(a.Items, b.Detail)
	if matched == 0 || estimate(a) <= ceiling {
		return a
	}

	// Thinning first: a name a caller can act on outlives a sentence it
	// was not going to read.
	a.Items = undocumented(a.Items)
	if estimate(a) <= ceiling {
		return truncate(a, matched)
	}

	// Then the source text. It is the largest thing an item carries, and
	// a caller holding a span can still read it.
	a.Items = unsnipped(a.Items)
	if estimate(a) <= ceiling {
		return truncate(a, matched)
	}

	// One item at a minimum. Zero beside a count reads like an answer
	// nothing served.
	for len(a.Items) > 1 && estimate(a) > ceiling {
		a.Items = a.Items[:len(a.Items)-1]
	}
	return truncate(a, matched)
}

// project returns the items carrying only what the detail level does.
func project(items []sema.Symbol, d Detail) []sema.Symbol {
	out := make([]sema.Symbol, len(items))
	for i, s := range items {
		switch d {
		case Summary:
			// What identifies the declaration and which file holds it.
			// The offsets, the parent and the visibility are what
			// Standard adds.
			out[i] = sema.Symbol{
				ID:       s.ID,
				Name:     s.Name,
				Kind:     s.Kind,
				Language: s.Language,
				Span:     source.Span{Path: s.Span.Path},
			}
		case Full:
			out[i] = s
		default:
			s.Doc, s.Snippet = "", ""
			out[i] = s
		}
	}
	return out
}

// undocumented returns the items with their documentation removed, which
// is the first thing the budget takes and the last a caller misses.
func undocumented(items []sema.Symbol) []sema.Symbol {
	out := make([]sema.Symbol, len(items))
	for i, s := range items {
		s.Doc = ""
		out[i] = s
	}
	return out
}

// unsnipped returns the items with their source text removed. A caller
// keeps the span, so what was dropped is still one read away.
func unsnipped(items []sema.Symbol) []sema.Symbol {
	out := make([]sema.Symbol, len(items))
	for i, s := range items {
		s.Snippet = ""
		out[i] = s
	}
	return out
}

// truncate records what was cut, when anything was.
func truncate(a engine.Answer[sema.Symbol], matched int) engine.Answer[sema.Symbol] {
	if len(a.Items) == matched {
		return a
	}
	a.Provenance.Caveats = append(a.Provenance.Caveats, trust.Caveat{
		Code: trust.CaveatTruncated,
		Note: fmt.Sprintf("%d matched, %d returned", matched, len(a.Items)),
	})
	return a
}

// estimate reports roughly what an answer will cost a caller.
//
// A value that will not serialise cannot be sent either, so it is
// treated as too large and thinned rather than returned whole.
func estimate(a engine.Answer[sema.Symbol]) int {
	encoded, err := json.Marshal(a)
	if err != nil {
		return int(^uint(0) >> 1)
	}
	return len(encoded) / bytesPerToken
}
