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

// addressed outlines a scope and picks the single declaration a name and a
// kind address.
//
// Every tool that names a declaration addresses it the same way, so the
// rule lives here rather than once per tool. A tool holding its own copy
// would be a tool where a name means something slightly different.
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
			Code: trust.Unsupported.String(),
			Reason: fmt.Sprintf(
				"nothing outlines %q, so no declaration could be found in it", scope),
		}
	}
	return pick(answered.Items, scope, name, kind)
}

// pick is the half of addressed that needs no service, so a tool that already
// has an outline does not fetch a second.
func pick(items []sema.Symbol, scope source.Path, name string, kind sema.Kind) (sema.Symbol, *Failure) {
	found := matching(items, name, kind)
	if len(found) == 1 {
		return found[0], nil
	}
	return sema.Symbol{}, &Failure{
		Code:   trust.Refused.String(),
		Reason: ambiguous(name, scope, found, items),
	}
}

// matching keeps the declarations a name and a kind pick out.
//
// A name matches as the language writes it, so Kind.Declares finds the
// method and Declares finds it too. A kind narrows what the name leaves
// ambiguous, and no kind matches every one.
func matching(found []sema.Symbol, name string, kind sema.Kind) []sema.Symbol {
	named := map[sema.ID]string{}
	for _, s := range found {
		named[s.ID] = s.Name
	}

	var out []sema.Symbol
	for _, s := range found {
		if kind != sema.KindUnknown && s.Kind != kind {
			continue
		}
		qualified := s.Name
		if parent, held := named[s.Parent]; held && s.Parent != "" {
			qualified = parent + "." + s.Name
		}
		if s.Name == name || qualified == name {
			out = append(out, s)
		}
	}
	return out
}

// ambiguous says why a name did not pick out one declaration.
//
// A name nothing matches is answered with what is nearby, and a name
// several match is answered with all of them: an agent that is told only
// "not found" retries with the same word, and one told the ten names in
// the file corrects itself in the same turn.
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

// sites lists where declarations were found, so a caller can tell them
// apart.
func sites(found []sema.Symbol) []string {
	out := make([]string, 0, len(found))
	for _, s := range found {
		out = append(out, fmt.Sprintf("%s at %s:%d", s.Kind, s.Span.Path, s.Span.Start.Line+1))
	}
	return out
}

// similar names the declarations a mistyped name was probably meant to
// be, and caps the list so a wrong name in a large scope does not answer
// with the whole scope.
//
// A name is offered back when it holds the one asked for, or when the
// two agree for their first few characters. Offering back every name the
// query happens to contain would answer a typo in normalizeBody with
// every one-letter local in the file, which is worse than answering
// nothing.
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

// agree returns how many leading bytes two names have in common.
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
	// nearLimit caps the names offered back for one that matched
	// nothing.
	nearLimit = 8
	// nearPrefix is how much of a name has to agree with the one asked
	// for before it is worth offering back.
	nearPrefix = 4
)
