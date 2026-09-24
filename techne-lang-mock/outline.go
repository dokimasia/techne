// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// dynamic is the caveat on every answer of an engine at the resolved tier.
var dynamic = trust.Caveat{
	Code: trust.CaveatDynamic,
	Note: "no static analysis binds a name that a program builds at run time",
}

// Outline returns the declarations of the files of the language in the scope of req, in path
// order and in the order that each file writes them. A declaration spans the lines nested
// under it.
func (e *Engine) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	w, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	return result(e, w.symbols, w, req.Scope), nil
}

// Search returns the declarations in the scope of req that match q. The exact names come
// first, then the names that start with the text of q, then the names and the documentation
// that contain it, each group in outline order and each match without regard to case. q.Kind,
// q.Private and q.Include filter the declarations before q.Limit cuts them. An answer cut at
// q.Limit has a [trust.CaveatTruncated] caveat with the number of matches returned and found.
func (e *Engine) Search(
	ctx context.Context,
	req engine.Request,
	q engine.Query,
) (engine.Result[sema.Symbol], error) {
	w, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}

	wanted := strings.ToLower(q.Text)
	locals := sema.Locals(w.symbols, sema.Containers(w.symbols))
	var exact, prefixed, loose []sema.Symbol
	for i, one := range w.symbols {
		if q.Kind != sema.KindUnknown && one.Kind != q.Kind || !q.Private && one.Visibility == sema.Unexported ||
			!q.Include.Keeps(one.Kind, locals[i]) {
			continue
		}
		name := strings.ToLower(one.Name)
		switch {
		case name == wanted:
			exact = append(exact, one)
		case strings.HasPrefix(name, wanted):
			prefixed = append(prefixed, one)
		case strings.Contains(name, wanted) || strings.Contains(strings.ToLower(one.Doc), wanted):
			loose = append(loose, one)
		}
	}

	matched := slices.Concat(exact, prefixed, loose)
	out := result(e, matched, w, req.Scope)
	if q.Limit > 0 && len(matched) > q.Limit {
		out.Items = matched[:q.Limit]
		out.Caveats = append(out.Caveats, trust.Caveat{
			Code: trust.CaveatTruncated,
			Note: fmt.Sprintf("%d of %d matches returned", q.Limit, len(matched)),
		})
	}
	return out, nil
}

// workspace is one read of the files of the language in a scope.
type workspace struct {
	// symbols are the declarations of the files that the read parsed, in path order.
	symbols []sema.Symbol
	// lines are the lines of each file that the read parsed.
	lines map[source.Path][]Line
	// content is the content of each file that the read parsed.
	content map[source.Path][]byte
	// claimed are the files of the language in the scope, the test files and the files that the
	// read leaves out included.
	claimed []source.Path
	// unread are the files larger than [lang.Largest], which the read leaves out.
	unread []source.Path
}

// claims reports whether scope contains a file of the language that the read found.
func (w workspace) claims(scope source.Path) bool {
	return slices.ContainsFunc(w.claimed, func(p source.Path) bool { return lang.Within(p, scoped(scope)) })
}

// read returns the files of the language in the scope of req, parsed, with the test files only
// when req includes tests. It walks the scope with [lang.Walk] and reads each file anew. A file
// larger than [lang.Largest] is not read.
func (e *Engine) read(ctx context.Context, req engine.Request) (workspace, error) {
	files, err := lang.Walk(e.fsys, scoped(req.Scope), e.declared.Extensions)
	if err != nil {
		return workspace{}, err
	}

	out := workspace{
		lines:   map[source.Path][]Line{},
		content: map[source.Path][]byte{},
		claimed: slices.Concat(files.Read, files.Unread),
		unread:  files.Unread,
	}
	for _, p := range files.Read {
		if err := ctx.Err(); err != nil {
			return workspace{}, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		content, err := fs.ReadFile(e.fsys, string(p))
		if err != nil {
			return workspace{}, fmt.Errorf("mock: read %s: %w", p, err)
		}
		lines, _ := Parse(p, content)
		out.lines[p], out.content[p] = lines, content
		out.symbols = append(out.symbols, e.declarations(p, lines, content)...)
	}
	return out, nil
}

// scoped returns scope, with the empty scope as [engine.Root].
func scoped(scope source.Path) source.Path {
	if scope == "" {
		return engine.Root
	}
	return scope
}

// declarations returns the declarations of the lines of the file at p. A declaration spans the
// lines nested under it. Its parent is the smallest declaration whose span contains it, which
// [sema.Containers] finds, and its ID is qualified by the qualified name of the parent.
func (e *Engine) declarations(p source.Path, lines []Line, content []byte) []sema.Symbol {
	var out []sema.Symbol
	for i, one := range lines {
		if one.Name == "" {
			continue
		}
		span := one.Span
		span.End = through(lines, i).End
		out = append(out, sema.Symbol{
			Name:       one.Name,
			Kind:       one.Kind,
			Language:   e.declared.Language,
			Span:       span,
			Visibility: e.declared.Visibility(one.Name),
			Doc:        one.Doc,
			Signature:  text(one),
			Snippet:    quoted(content, span),
		})
	}

	// A parent starts on an earlier line than its child, so its qualified name and its ID are
	// set before the child reads them.
	unit := source.Path(e.declared.Namespace(string(p)))
	qualified := make([]string, len(out))
	for i, parent := range sema.Containers(out) {
		qualified[i] = out[i].Name
		if parent >= 0 {
			qualified[i] = sema.Qualify(qualified[parent], out[i].Name)
			out[i].Parent = out[parent].ID
		}
		out[i].ID = sema.NewID(e.declared.Language, unit, qualified[i], out[i].Kind)
	}
	return out
}

// through returns the span of the last line nested under the line at index at, or of that line
// when nothing is nested under it.
func through(lines []Line, at int) source.Span {
	last := lines[at].Span
	for i := at + 1; i < len(lines); i++ {
		if lines[i].Depth <= lines[at].Depth {
			break
		}
		last = lines[i].Span
	}
	return last
}

// text returns a line without its indentation: the word and the name.
func text(one Line) string {
	if one.Uses != "" {
		return refers + " " + one.Uses
	}
	return kindWord(one.Kind) + " " + one.Name
}

// kindWord returns the word that declares a declaration of kind k.
func kindWord(k sema.Kind) string {
	for word, declared := range declares {
		if declared == k {
			return word
		}
	}
	return ""
}

// result returns items with the evidence of w for scope:
//
//   - The result is skipped when scope contains no file of the language.
//   - The completeness is the one that [Covering] set, and partial when the read left out a
//     file larger than [lang.Largest]. A [trust.CaveatUnread] caveat lists such files.
//   - An engine at the resolved tier adds the [trust.CaveatDynamic] caveat.
func result[T any](e *Engine, items []T, w workspace, scope source.Path) engine.Result[T] {
	out := engine.Result[T]{Items: items, Completeness: e.coverage, Skipped: !w.claims(scope)}
	if e.fidelity >= trust.Resolved {
		out.Caveats = append(out.Caveats, dynamic)
	}
	if len(w.unread) > 0 {
		if out.Completeness == trust.ScopeTotal {
			out.Completeness = trust.ScopePartial
		}
		out.Caveats = append(out.Caveats, trust.Caveat{
			Code:  trust.CaveatUnread,
			Note:  fmt.Sprintf("larger than %d bytes, so not read", lang.Largest),
			Paths: w.unread,
		})
	}
	return out
}
