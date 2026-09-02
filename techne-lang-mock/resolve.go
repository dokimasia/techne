// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Resolve reports what the name at a position denotes.
//
// The position may carry only a line and a column, which is what a
// caller reading an editor has. The offset is worked out here, because
// only an engine has the file to count in.
//
// The whole workspace is read, not the file the position is in. What a
// name denotes is usually declared somewhere else, and an engine that
// looked only where it was pointed would answer "nothing" for every
// cross-file reference while claiming it had covered the scope.
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	held, err := e.read(ctx, everywhere(req))
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}

	named := naming(held, req.Scope, at)
	if named == "" {
		return e.found(nil, len(held.lines)), nil
	}

	var out []sema.Symbol
	for _, one := range held.symbols {
		if one.Name == named {
			out = append(out, one)
		}
	}
	return e.found(out, len(held.lines)), nil
}

// Relate reports how a declaration connects to the rest.
//
// One direction is stored and both are answered: a use line is an edge
// from what holds it to what it names, so who calls this and what this
// calls are the same edges read from opposite ends.
//
// The whole workspace, for the reason [Engine.Resolve] reads it: who
// calls this is a question about everywhere, and an engine that read one
// file would report the callers in it and claim there were no others.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	held, err := e.read(ctx, everywhere(req))
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}

	subject, known := byID(held.symbols, of)
	if !known {
		return engine.Result[sema.Relation]{Completeness: e.coverage}, nil
	}

	var out []sema.Relation
	switch kind {
	case sema.CalledBy, sema.ReferencedBy:
		out = e.toward(held, subject, kind)
	case sema.Calls, sema.References:
		out = from(held, subject, kind)
	default:
		// A direction this language has no edge for is answered with
		// none rather than refused: the caller asked something
		// answerable and the answer is that there are none.
	}
	return engine.Result[sema.Relation]{Items: out, Completeness: e.coverage}, nil
}

// toward finds what refers to a declaration.
func (e *Engine) toward(held workspace, of sema.Symbol, kind sema.RelationKind) []sema.Relation {
	var out []sema.Relation
	for _, p := range slices.Sorted(maps.Keys(held.lines)) {
		lines := held.lines[p]
		for i, one := range lines {
			if one.Uses != of.Name {
				continue
			}
			within, found := enclosing(lines, i)
			if !found {
				continue
			}
			out = append(out, sema.Relation{
				Kind: kind,
				To:   e.symbol(p, held, within),
				At:   one.At,
				Via:  text(one),
			})
		}
	}
	return sorted(out)
}

// from finds what a declaration refers to.
func from(held workspace, of sema.Symbol, kind sema.RelationKind) []sema.Relation {
	var out []sema.Relation
	for _, p := range slices.Sorted(maps.Keys(held.lines)) {
		lines := held.lines[p]
		for i, one := range lines {
			if one.Uses == "" {
				continue
			}
			within, found := enclosing(lines, i)
			if !found || within.Name != of.Name {
				continue
			}
			for _, named := range held.symbols {
				if named.Name != one.Uses {
					continue
				}
				out = append(out, sema.Relation{
					Kind: kind, To: named, At: one.At, Via: text(one),
				})
			}
		}
	}
	return sorted(out)
}

// sorted puts edges in a fixed order, so two identical questions get two
// identical answers and a caller diffing them sees only changes somebody
// made.
func sorted(edges []sema.Relation) []sema.Relation {
	slices.SortStableFunc(edges, func(a, b sema.Relation) int {
		if held := strings.Compare(string(a.At.Path), string(b.At.Path)); held != 0 {
			return held
		}
		return a.At.Start.Offset - b.At.Start.Offset
	})
	return edges
}

// enclosing is the declaration a line sits inside: the nearest one above
// it at a shallower depth.
func enclosing(lines []Line, at int) (Line, bool) {
	for i := at - 1; i >= 0; i-- {
		if lines[i].Name != "" && lines[i].Depth < lines[at].Depth {
			return lines[i], true
		}
	}
	return Line{}, false
}

// symbol is one line as the declaration it makes.
func (e *Engine) symbol(p source.Path, held workspace, one Line) sema.Symbol {
	for _, named := range e.declarations(p, held.lines[p], held.content[p]) {
		if named.Name == one.Name && named.Span == one.Span {
			return named
		}
	}
	return sema.Symbol{Name: one.Name, Kind: one.Kind, Language: e.declared.Language}
}

// byID finds the declaration an identity names, and reports whether
// exactly one does.
func byID(items []sema.Symbol, of sema.ID) (sema.Symbol, bool) {
	var found sema.Symbol
	seen := 0
	for _, one := range items {
		if one.ID == of {
			found, seen = one, seen+1
		}
	}
	return found, seen == 1
}

// everywhere widens a request to the workspace.
//
// A question about what a name means is a question about everywhere it
// could have been declared. The scope a caller gave still says where the
// position is; it does not say where the answer may come from.
func everywhere(req engine.Request) engine.Request {
	req.Scope = engine.Root
	req.Tests = true
	return req
}

// naming is the declaration a position falls on.
//
// A map ranges in no order, so the walk is over the file the caller
// asked about rather than over whatever came first.
func naming(held workspace, scope source.Path, where source.Position) string {
	for _, p := range slices.Sorted(maps.Keys(held.lines)) {
		lines := held.lines[p]
		if scope != "" && p != scope {
			continue
		}
		for _, one := range lines {
			if one.At.Start.Line != where.Line {
				continue
			}
			if where.Column >= one.At.Start.Column && where.Column < one.At.End.Column {
				if one.Uses != "" {
					return one.Uses
				}
				return one.Name
			}
		}
	}
	return ""
}
