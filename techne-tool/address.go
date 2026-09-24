// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// addressed returns the declaration that a name and a kind address in the outline of scope,
// with the test files of the scope. The document.symbol, rename.symbol and relations tools
// call it, so a name addresses the same declaration in each of them.
//
// It returns a refused [Failure] when the name addresses no declaration or more than one, and
// an unsupported Failure when no engine outlines the scope. A name that addresses nothing in a
// scope with a file larger than an engine reads is refused with the name of that file, because
// the declaration can be in it.
func addressed(
	ctx context.Context,
	reads Outliner,
	req engine.Request,
	scope source.Path,
	name string,
	kind sema.Kind,
) (sema.Symbol, *Failure) {
	asking := req
	asking.Tests = true
	answered, err := reads.Outline(ctx, asking)
	if err != nil {
		return sema.Symbol{}, &Failure{
			Code:   trust.Refused.String(),
			Reason: fmt.Sprintf("%q could not be read: %v", scope, err),
		}
	}
	if !answered.Status.Answered() {
		return sema.Symbol{}, &Failure{
			Code:   trust.Unsupported.String(),
			Reason: fmt.Sprintf("no engine outlines %q, so no declaration in it can be found", scope),
		}
	}
	found, failure := pick(answered.Items, scope, name, kind)
	if failure != nil && len(matching(answered.Items, name, kind)) == 0 {
		if unread, skipped := passedOver(answered.Provenance); skipped {
			return sema.Symbol{}, &Failure{
				Code: trust.Refused.String(),
				Reason: fmt.Sprintf(
					"%q was not read in full: %s is larger than an engine reads, "+
						"so %q was not looked for there",
					scope, unread, name),
			}
		}
	}
	return found, failure
}

// passedOver returns the files of the unread caveat of p, and reports whether p has one.
func passedOver(p trust.Provenance) (string, bool) {
	for _, one := range p.Caveats {
		if one.Code == trust.CaveatUnread {
			return strings.Join(paths(one.Paths), ", "), true
		}
	}
	return "", false
}

// paths returns each path of list as a string.
func paths(list []source.Path) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, string(p))
	}
	return out
}

// pick returns the declaration of items that a name and a kind address, for a tool that has
// the outline already. The declarations that [matching] selects address one declaration when
// there is one of them, or when [together] reports that they are one subject. Any other count
// is refused with the reason of [ambiguous].
func pick(items []sema.Symbol, scope source.Path, name string, kind sema.Kind) (sema.Symbol, *Failure) {
	found := matching(items, name, kind)
	if len(found) == 1 {
		return found[0], nil
	}
	if one, same := together(found); same {
		return one, nil
	}
	return sema.Symbol{}, &Failure{
		Code:   trust.Refused.String(),
		Reason: ambiguous(name, scope, found, items),
	}
}

// together returns the declaration that two or more declarations of one name address, and
// reports whether they address one. They do in these cases:
//
//   - They are imports of one name, which bring one thing into scope in each of their files.
//     The first import is the declaration.
//   - One of them is a type and the others are constructors that the type contains, as Java
//     and C# name a constructor after its type. The type is the declaration.
//   - They share one ID, as the prototype and the definition of a C function do and the
//     overload signatures of a TypeScript function do. The first in the order of the outline
//     is the declaration.
func together(found []sema.Symbol) (sema.Symbol, bool) {
	if len(found) < 2 {
		return sema.Symbol{}, false
	}
	if every(found, func(s sema.Symbol) bool { return s.Kind == sema.KindImport && s.Name == found[0].Name }) {
		return found[0], true
	}
	if built, is := constructed(found); is {
		return built, true
	}
	if every(found, func(s sema.Symbol) bool { return s.ID == found[0].ID }) {
		return found[0], true
	}
	return sema.Symbol{}, false
}

// constructed returns the one declaration of found that is not a constructor, and reports
// whether every other declaration is a constructor that it contains.
func constructed(found []sema.Symbol) (sema.Symbol, bool) {
	var built []sema.Symbol
	for _, s := range found {
		if s.Kind != sema.KindConstructor {
			built = append(built, s)
		}
	}
	if len(built) != 1 {
		return sema.Symbol{}, false
	}
	return built[0], every(found, func(s sema.Symbol) bool {
		return s.Kind != sema.KindConstructor || s.Parent == built[0].ID
	})
}

// every reports whether rule reports true for each declaration of found.
func every(found []sema.Symbol, rule func(sema.Symbol) bool) bool {
	for _, s := range found {
		if !rule(s) {
			return false
		}
	}
	return true
}

// matching returns the declarations of found that name and kind select. A name selects a
// declaration of that name, or of that qualified name in its ID, such as Store.Get for the
// method Get of Store. [sema.KindUnknown] selects every kind.
func matching(found []sema.Symbol, name string, kind sema.Kind) []sema.Symbol {
	var out []sema.Symbol
	for _, s := range found {
		if kind != sema.KindUnknown && s.Kind != kind {
			continue
		}
		if s.Name == name || s.ID.Name() == name {
			out = append(out, s)
		}
	}
	return out
}

// ambiguous returns the reason that a name addresses no declaration or more than one. The
// reason for more than one lists the kind and the site of each. The reason for none lists the
// names of scope that [similar] returns.
func ambiguous(name string, scope source.Path, found, all []sema.Symbol) string {
	if len(found) > 1 {
		return fmt.Sprintf("%q names %d declarations in %q: %s — narrow it with kind, "+
			"or qualify it as the language writes it",
			name, len(found), scope, strings.Join(sites(found), ", "))
	}
	near := similar(all, name)
	if len(near) == 0 {
		return fmt.Sprintf("%q declares nothing called %q", scope, name)
	}
	return fmt.Sprintf("%q declares nothing called %q. It declares %s",
		scope, name, strings.Join(near, ", "))
}

// sites returns the kind, the path and the line of each declaration of found, the line counted
// from one.
func sites(found []sema.Symbol) []string {
	out := make([]string, 0, len(found))
	for _, s := range found {
		out = append(out, fmt.Sprintf("%s at %s:%d", s.Kind, s.Span.Path, s.Span.Start.Line+1))
	}
	return out
}

// similar returns up to [nearLimit] names of all that name can be a misspelling of: a name
// that contains name, or that starts with the same [nearPrefix] bytes, both without regard to
// case. It returns only names that [sema.Kind.Declares] allows.
func similar(all []sema.Symbol, name string) []string {
	var out []string
	seen := map[string]bool{}
	folded := strings.ToLower(name)
	for _, s := range all {
		if !s.Kind.Declares() || seen[s.Name] {
			continue
		}
		low := strings.ToLower(s.Name)
		if !strings.Contains(low, folded) && agree(low, folded) < nearPrefix {
			continue
		}
		seen[s.Name] = true
		out = append(out, s.Name)
		if len(out) == nearLimit {
			break
		}
	}
	return out
}

// agree returns the number of leading bytes that a and b have in common.
func agree(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

const (
	// nearLimit is the largest number of names that [similar] returns.
	nearLimit = 8
	// nearPrefix is the number of leading bytes that a name shares with the name asked for,
	// for [similar] to return it.
	nearPrefix = 4
)
