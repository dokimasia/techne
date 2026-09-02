// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Outline reports what the files in a scope declare.
func (e *Engine) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	held, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	return e.found(held.symbols, len(held.lines)), nil
}

// Search reports the declarations in a scope matching a query.
//
// An exact name first, then a prefix, then anything holding the text.
// The order is the engine's own, which is what a service must not
// re-rank: an engine that binds names knows more about which match is
// wanted than the thing displaying them.
func (e *Engine) Search(
	ctx context.Context,
	req engine.Request,
	q engine.Query,
) (engine.Result[sema.Symbol], error) {
	held, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}

	wanted := strings.ToLower(q.Text)
	var exact, prefixed, loose []sema.Symbol
	for _, one := range held.symbols {
		if q.Kind != sema.KindUnknown && one.Kind != q.Kind {
			continue
		}
		if !q.Private && one.Visibility == sema.Unexported {
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

	out := append(append(exact, prefixed...), loose...)
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return e.found(out, len(held.lines)), nil
}

// workspace is one read of a scope: what it declares, and what refers to
// what.
type workspace struct {
	symbols []sema.Symbol
	// lines are the parsed lines each file holds, kept so the write path
	// can point at a name rather than at a line.
	lines map[source.Path][]Line
	// uses are the reference sites, by the name they name.
	uses map[string][]source.Span
	// broken are the lines that are not this language.
	broken []source.Span
	// content is what each file held, kept so a plan can be computed
	// against the bytes rather than against the lines alone.
	content map[source.Path][]byte
}

// read walks a scope and reads every file this language claims.
//
// The whole scope, every time. An engine that cached would be an engine
// whose staleness had to be tested, and what this exists for is the
// tools rather than the caching.
func (e *Engine) read(ctx context.Context, req engine.Request) (workspace, error) {
	paths, err := lang.FilesIn(e.fsys, req.Scope, e.declared.Extensions)
	if err != nil {
		return workspace{}, err
	}

	out := workspace{
		lines:   map[source.Path][]Line{},
		uses:    map[string][]source.Span{},
		content: map[source.Path][]byte{},
	}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return workspace{}, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		content, readErr := fs.ReadFile(e.fsys, string(p))
		if readErr != nil {
			return workspace{}, fmt.Errorf("mock: read %s: %w", p, readErr)
		}

		lines, broken := Parse(p, content)
		out.lines[p], out.content[p] = lines, content
		out.broken = append(out.broken, broken...)
		out.symbols = append(out.symbols, e.declarations(p, lines, content)...)
		for _, one := range lines {
			if one.Uses != "" {
				out.uses[one.Uses] = append(out.uses[one.Uses], one.At)
			}
		}
	}
	return out, nil
}

// declarations turns one file's lines into the declarations they make.
//
// Nesting is by indentation, so the parent of a line is the nearest line
// above it at a shallower depth. A use nests under whatever it sits in
// and declares nothing.
func (e *Engine) declarations(p source.Path, lines []Line, content []byte) []sema.Symbol {
	unit := source.Path(e.declared.Namespace(string(p)))

	var out []sema.Symbol
	within := map[int]sema.ID{}
	for i, one := range lines {
		if one.Name == "" {
			continue
		}
		id := sema.NewID(e.declared.Language, unit, one.Name, one.Kind)
		within[one.Depth] = id

		// A declaration covers what is nested under it, as it does in
		// every language with a body. Reporting the line alone would
		// leave nothing able to tell that a field sits inside a type.
		span := one.Span
		span.End = through(lines, i).End

		held := sema.Symbol{
			ID:         id,
			Name:       one.Name,
			Kind:       one.Kind,
			Language:   e.declared.Language,
			Span:       span,
			Visibility: e.declared.Visibility(one.Name),
			Doc:        one.Doc,
			Signature:  text(one),
			Snippet:    quoted(content, span),
		}
		if one.Depth > 0 {
			held.Parent = within[one.Depth-1]
		}
		out = append(out, held)
	}
	return out
}

// through is the last line nested under the one at this index, or that
// line itself where nothing is.
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

// text is the declaration as written, without its indentation.
func text(one Line) string {
	if one.Uses != "" {
		return refers + " " + one.Uses
	}
	return kindWord(one.Kind) + " " + one.Name
}

// kindWord is the word this language declares that kind with.
func kindWord(k sema.Kind) string {
	for word, held := range declares {
		if held == k {
			return word
		}
	}
	return ""
}

// found wraps symbols in the result this engine returns, at the tier it
// was registered to claim.
//
// A scope holding no file of this language says so. It is not an answer
// about the language, and a service merging several must not let it
// lower what the others are worth.
func (e *Engine) found(items []sema.Symbol, read int) engine.Result[sema.Symbol] {
	out := engine.Result[sema.Symbol]{
		Items: items, Completeness: e.coverage, Skipped: read == 0,
	}
	if e.fidelity >= trust.Resolved {
		// Every resolved answer carries it, because no static analysis
		// sees a name assembled at run time.
		out.Caveats = []trust.Caveat{{
			Code: trust.CaveatDynamic,
			Note: "a name built at run time is invisible here, as it is to every engine",
		}}
	}
	return out
}
