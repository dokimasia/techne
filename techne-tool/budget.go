// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Detail is how much of each item an answer carries.
//
// A level is named for what it holds rather than for how big it is, so
// an agent chooses one without spending a call to learn that the
// smallest drops the line numbers. Each contains the ones above it.
//
// The zero value is [DetailUnset], and the default follows the scope: a
// file is asked about to work in it, and a directory to find the right
// file.
type Detail string

const (
	// DetailUnset means the caller named no level.
	DetailUnset Detail = ""
	// Names identifies a declaration and says where it is.
	Names Detail = "names"
	// Signatures adds what a caller needs to call it, implement it or
	// match it, and nothing of how it works.
	Signatures Detail = "signatures"
	// Docs adds the documentation comment.
	Docs Detail = "docs"
	// Source adds the declaration's own text and the bytes it covers.
	Source Detail = "source"
)

// Levels returns every level, cheapest first.
func Levels() []Detail { return []Detail{Names, Signatures, Docs, Source} }

// DefaultDetail is the level for a scope the caller named no level for.
//
// A file is asked about because someone means to work in it, and
// signatures is where the answer replaces reading it. A directory is
// asked about to find the right file, which names answer for a fraction
// of the cost.
func DefaultDetail(scope source.Path) Detail {
	if !names(scope) {
		return Names
	}
	return Signatures
}

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
//
// It names no level. A level decides what an item carries and is applied
// where the answer is built; the budget removes what an item already
// carries, which is the same thing done later and to fewer fields.
type Budget struct {
	// MaxTokens is the estimated ceiling. Zero means
	// [DefaultMaxTokens].
	MaxTokens int
}

// Fit returns the answer thinned to the budget.
//
// The order removes the cheapest evidence first: prose, then source
// text, then the bindings that never leave their scope, then members,
// then what is not visible outside its unit, and only then whole items.
// A caller that asked what a scope declares can act on a name alone, and
// the tail of a file is not its least valuable part.
//
// The provenance is never changed: thinning an answer says nothing about
// the evidence behind it.
func Fit(a Answer, b Budget) Answer {
	ceiling := b.MaxTokens
	if ceiling <= 0 {
		ceiling = DefaultMaxTokens
	}

	matched := count(a.Items)
	dropped := map[string]int{}
	// An answer already fitted says so, and fitting it again would thin
	// it for the cost of the caveat that says it was thinned.
	if matched == 0 || cut(a) || estimate(a) <= ceiling {
		return a
	}

	for _, thin := range []func([]Declaration) []Declaration{undocumented, unsnipped} {
		a.Items = thin(a.Items)
		if estimate(a) <= ceiling {
			return truncate(a, matched, dropped)
		}
	}

	// Kinds that bind a name never leaving their scope, cheapest first.
	for _, kind := range []sema.Kind{
		sema.KindLabel, sema.KindImport, sema.KindTypeParameter, sema.KindParameter,
	} {
		a.Items = without(a.Items, func(d Declaration) bool { return d.Kind == kind }, dropped)
		if estimate(a) <= ceiling {
			return truncate(a, matched, dropped)
		}
	}

	for depth := deepest(a.Items); depth > 0; depth-- {
		a.Items = shallower(a.Items, depth, dropped)
		if estimate(a) <= ceiling {
			return truncate(a, matched, dropped)
		}
	}

	a.Items = without(a.Items, func(d Declaration) bool {
		return d.Visibility == sema.Unexported
	}, dropped)
	if estimate(a) <= ceiling {
		return truncate(a, matched, dropped)
	}

	// One item at a minimum. Zero beside a count reads like an answer
	// nothing served.
	//
	// Found by halving rather than by dropping one at a time. Estimating
	// an answer renders it, so one drop per item is one render per item:
	// a search with no name to match returned thirty-five thousand
	// declarations and spent twelve seconds re-rendering what had not
	// changed. An answer with fewer items never estimates larger, so the
	// longest prefix that fits is found in as many estimates as the list
	// has bits.
	if len(a.Items) > 1 && estimate(a) > ceiling {
		whole := a.Items
		fits, most := 1, len(whole)
		for fits < most {
			half := (fits + most + 1) / 2
			a.Items = whole[:half]
			if estimate(a) <= ceiling {
				fits = half
			} else {
				most = half - 1
			}
		}
		a.Items = whole[:fits]
		for _, one := range whole[fits:] {
			dropped[one.Kind.String()] += count([]Declaration{one})
		}
	}
	return truncate(a, matched, dropped)
}

// cut reports whether an answer has already been fitted.
func cut(a Answer) bool {
	for _, c := range a.Provenance.Caveats {
		if c.Code == string(trust.CaveatTruncated) {
			return true
		}
	}
	return false
}

// without removes every declaration a rule names, at any depth.
func without(items []Declaration, rule func(Declaration) bool, dropped map[string]int) []Declaration {
	out := make([]Declaration, 0, len(items))
	for _, item := range items {
		if rule(item) {
			dropped[item.Kind.String()] += count([]Declaration{item})
			continue
		}
		item.Members = without(item.Members, rule, dropped)
		out = append(out, item)
	}
	return out
}

// shallower removes the members sitting at a depth, deepest first, so a
// tree loses its leaves before it loses a whole branch.
func shallower(items []Declaration, depth int, dropped map[string]int) []Declaration {
	out := make([]Declaration, len(items))
	for i, item := range items {
		if depth <= 1 {
			for _, member := range item.Members {
				dropped[member.Kind.String()] += count([]Declaration{member})
			}
			item.Members = nil
		} else {
			item.Members = shallower(item.Members, depth-1, dropped)
		}
		out[i] = item
	}
	return out
}

// deepest reports how far the tree goes.
func deepest(items []Declaration) int {
	most := 0
	for _, item := range items {
		if held := deepest(item.Members); held+1 > most {
			most = held + 1
		}
	}
	return most
}

// undocumented removes the prose, which is the first thing the budget
// takes and the last a caller misses.
func undocumented(items []Declaration) []Declaration {
	out := make([]Declaration, len(items))
	for i, item := range items {
		item.Doc = ""
		item.Members = undocumented(item.Members)
		out[i] = item
	}
	return out
}

// unsnipped removes the source text. A caller keeps the line, so what
// was dropped is one read away.
func unsnipped(items []Declaration) []Declaration {
	out := make([]Declaration, len(items))
	for i, item := range items {
		item.Snippet = ""
		item.Members = unsnipped(item.Members)
		out[i] = item
	}
	return out
}

// truncate records what was cut, by kind, when anything was.
func truncate(a Answer, matched int, dropped map[string]int) Answer {
	held := count(a.Items)
	if held == matched {
		return a
	}
	note := fmt.Sprintf("%d matched, %d returned", matched, held)
	if by := sorted(dropped); by != "" {
		note += "; dropped " + by
	}
	a.Provenance.Caveats = append(a.Provenance.Caveats, Caveat{
		Code: string(trust.CaveatTruncated),
		Note: note,
	})
	return a
}

// sorted renders what went, so a caller reads which kinds it lost rather
// than only how many items.
func sorted(dropped map[string]int) string {
	kinds := make([]string, 0, len(dropped))
	for kind := range dropped {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(a, b int) bool { return dropped[kinds[a]] > dropped[kinds[b]] })

	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", dropped[kind], kind))
	}
	return strings.Join(parts, ", ")
}

// estimate reports roughly what an answer will cost a caller.
//
// The rendered form is what a model reads, so that is what is measured.
// A value that will not serialise cannot be sent either, so it is
// treated as too large and thinned rather than returned whole.
func estimate(a Answer) int {
	if _, err := json.Marshal(a); err != nil {
		return int(^uint(0) >> 1)
	}
	return len(a.Render()) / bytesPerToken
}
