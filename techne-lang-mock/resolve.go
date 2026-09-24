// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// Resolve returns the declarations of every file of the workspace that the name at a position
// names. The scope of req is the file of the position, and a use can refer to a declaration of
// any file. A position with an offset takes its line and column from the file.
//
// Resolve returns a skipped result for a scope without a file of the language. It declines a
// directory with files of the language, because a position belongs to one file, and a file of
// the language that the workspace does not contain or that is too large to read.
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	w, err := e.read(ctx, everywhere(req))
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	p := scoped(req.Scope)
	lines, parsed := w.lines[p]
	switch {
	case parsed:
	case lang.Claims(string(p), e.declared.Extensions) && slices.Contains(w.unread, p):
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: mock: %s is larger than %d bytes",
			engine.ErrDecline, p, lang.Largest)
	case lang.Claims(string(p), e.declared.Extensions):
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: mock: the workspace contains no %s",
			engine.ErrDecline, p)
	case w.claims(p):
		return engine.Result[sema.Symbol]{}, fmt.Errorf(
			"%w: mock: a position names a file, and %s is a directory", engine.ErrDecline, p)
	default:
		return result(e, []sema.Symbol(nil), w, p), nil
	}

	line, column := at.Line, at.Column
	if at.Offset > 0 {
		line, column = placed(w.content[p], at.Offset)
	}
	named := naming(lines, line, column)
	var out []sema.Symbol
	for _, one := range w.symbols {
		if named != "" && one.Name == named {
			out = append(out, one)
		}
	}
	return result(e, out, w, p), nil
}

// Relate returns the relations of kind from the declaration with the ID of, from every file of
// the workspace, sorted by the path and the offset of their sites. A use is an edge from the
// innermost declaration that contains it to each declaration of the name that it uses:
//
//   - [sema.ReferencedBy] and [sema.CalledBy] return the uses of the name of the declaration.
//   - [sema.References] and [sema.Calls] return the declarations that the uses inside the
//     declaration name.
//
// Another kind gets an empty result, because the language has no other edge. Relate returns a
// skipped result for a scope without a file of the language. It declines an ID that no
// declaration has, and refuses an ID that two or more declarations have.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	w, err := e.read(ctx, everywhere(req))
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}
	if !w.claims(req.Scope) {
		return result(e, []sema.Relation(nil), w, req.Scope), nil
	}
	found := matching(w.symbols, of)
	switch len(found) {
	case 0:
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: mock: no declaration matches %s",
			engine.ErrDecline, of)
	case 1:
	default:
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: mock: %s names %d declarations",
			engine.ErrRefuse, of, len(found))
	}

	var out []sema.Relation
	switch kind {
	case sema.CalledBy, sema.ReferencedBy:
		out = toward(w, found[0], kind)
	case sema.Calls, sema.References:
		out = from(w, found[0], kind)
	}
	return result(e, out, w, req.Scope), nil
}

// toward returns the uses of the name of the declaration subject, each with the innermost
// declaration that contains it, in path order and in the order of the lines of each file.
func toward(w workspace, subject sema.Symbol, kind sema.RelationKind) []sema.Relation {
	var out []sema.Relation
	for _, p := range slices.Sorted(maps.Keys(w.lines)) {
		for _, one := range w.lines[p] {
			if one.Uses != subject.Name {
				continue
			}
			if container, found := innermost(w.symbols, one.At); found {
				out = append(out, sema.Relation{Kind: kind, To: container, At: one.At, Via: text(one)})
			}
		}
	}
	return out
}

// from returns the declarations that the uses inside the declaration subject name, at the site
// of each use, in path order and in the order of the lines of each file.
func from(w workspace, subject sema.Symbol, kind sema.RelationKind) []sema.Relation {
	var out []sema.Relation
	for _, p := range slices.Sorted(maps.Keys(w.lines)) {
		for _, one := range w.lines[p] {
			if one.Uses == "" {
				continue
			}
			if container, found := innermost(w.symbols, one.At); !found || container.ID != subject.ID {
				continue
			}
			for _, named := range w.symbols {
				if named.Name == one.Uses {
					out = append(out, sema.Relation{Kind: kind, To: named, At: one.At, Via: text(one)})
				}
			}
		}
	}
	return out
}

// innermost returns the declaration of symbols with the smallest span that contains at, and
// reports whether a span contains it. The spans of one file nest, so the smallest span that
// contains at is the one that starts last.
func innermost(symbols []sema.Symbol, at source.Span) (sema.Symbol, bool) {
	var out sema.Symbol
	found := false
	for _, one := range symbols {
		inside := one.Span.Path == at.Path && one.Span.Start.Offset <= at.Start.Offset &&
			at.End.Offset <= one.Span.End.Offset
		if inside && (!found || one.Span.Start.Offset > out.Span.Start.Offset) {
			out, found = one, true
		}
	}
	return out, found
}

// matching returns the declarations of items with the ID of.
func matching(items []sema.Symbol, of sema.ID) []sema.Symbol {
	var out []sema.Symbol
	for _, one := range items {
		if one.ID == of {
			out = append(out, one)
		}
	}
	return out
}

// everywhere returns req with the whole workspace as its scope and the test files included, for
// a question whose answer can be in any file.
func everywhere(req engine.Request) engine.Request {
	req.Scope = engine.Root
	req.Tests = true
	return req
}

// naming returns the name that the line of lines at the zero-based line and column covers, or
// the empty string for none.
func naming(lines []Line, line, column int) string {
	for _, one := range lines {
		if one.At.Start.Line != line || column < one.At.Start.Column || column >= one.At.End.Column {
			continue
		}
		if one.Uses != "" {
			return one.Uses
		}
		return one.Name
	}
	return ""
}

// placed returns the zero-based line and column of the byte at offset of content.
func placed(content []byte, offset int) (int, int) {
	offset = min(offset, len(content))
	start := bytes.LastIndexByte(content[:offset], '\n') + 1
	return bytes.Count(content[:offset], []byte("\n")), offset - start
}
