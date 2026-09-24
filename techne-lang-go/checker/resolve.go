// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/packages"
)

// Resolve returns the declaration that the name at a position denotes, with the source line of
// the declaration as its snippet. It returns no declaration for a position that is not on a
// name that the type checker bound.
//
// The scope of req is the file that contains the position. Resolve returns a skipped result for
// a scope without a Go file, and declines a directory with Go files, because a position belongs
// to one file. It declines a file outside every loaded package, such as a file that its build
// constraints exclude. A position with a line and a column and no offset gets its offset from
// the content of the file.
//
// An error on the line of the position lowers the answer, and so does any error of the program
// of the file when the answer is empty.
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	p := scoped(req.Scope)
	w, err := e.walk()
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !lang.Claims(string(p), e.declared.Extensions) {
		if !w.claims(p) {
			return engine.Result[sema.Symbol]{Skipped: true, Completeness: trust.ScopeTotal}, nil
		}
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: checker: a position names a file, and %s is a directory",
			engine.ErrDecline, p)
	}
	if unreadable := lang.Readable(os.DirFS(e.root), p); unreadable != nil {
		return engine.Result[sema.Symbol]{}, unreadable
	}
	content, err := os.ReadFile(e.fullPath(p))
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("checker: read %s: %w", p, err)
	}

	v, err := e.current(ctx, w)
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	pkg, file, compiled := e.holding(v, p)
	if !compiled {
		reason := ""
		if covered, missing := v.partial(p); covered != trust.ScopeTotal {
			reason = ", and " + missing[0].Note
		}
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: checker: no loaded package compiles %s%s",
			engine.ErrDecline, p, reason)
	}

	offset := at.Offset
	if offset <= 0 && (at.Line > 0 || at.Column > 0) {
		offset = byteAt(content, at.Line, at.Column)
	}
	var out []sema.Symbol
	if named := identAt(file, v.fset, offset); named != nil {
		of := pkg.TypesInfo.Uses[named]
		if of == nil {
			of = pkg.TypesInfo.Defs[named]
		}
		if found, declares := e.symbolFor(v, of); declares {
			found.Snippet = snippet(e.fullPathOf(found.Span.Path), found.Span.Start.Offset)
			out = append(out, found)
		}
	}

	line := bytes.Count(content[:min(max(offset, 0), len(content))], []byte("\n"))
	tier, caveats := e.lowered(v, p, lang.Binding(p, line, len(out) > 0))
	return engine.Result[sema.Symbol]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Lowered:      tier,
		Caveats:      slices.Concat([]trust.Caveat{dynamic}, caveats),
	}, nil
}

// holding returns the held package that compiles the file at p and the syntax of the file, and
// reports whether one does.
func (e *Engine) holding(v *view, p source.Path) (*packages.Package, *ast.File, bool) {
	full := e.fullPath(p)
	for _, pkg := range v.held() {
		for _, file := range pkg.Syntax {
			if v.fset.File(file.FileStart).Name() == full {
				return pkg, file, true
			}
		}
	}
	return nil, nil, false
}

// fullPathOf returns the absolute path of a path of an answer: a workspace path, or the slash
// form of the absolute path of a file outside the workspace.
func (e *Engine) fullPathOf(p source.Path) string {
	if lang.Within(p, engine.Root) {
		return e.fullPath(p)
	}
	return string(p)
}

// snippet returns the source line at offset of the file at full, without the white space
// around it, or the empty string for a file that cannot be read.
func snippet(full string, offset int) string {
	content, err := os.ReadFile(full)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(lang.LineAt(content, offset))
}

// identAt returns the identifier of file that contains offset, or nil for none.
func identAt(file *ast.File, fset *token.FileSet, offset int) *ast.Ident {
	var found *ast.Ident
	ast.Inspect(file, func(n ast.Node) bool {
		named, is := n.(*ast.Ident)
		if !is {
			return true
		}
		from := fset.Position(named.Pos()).Offset
		if offset >= from && offset < from+len(named.Name) {
			found = named
		}
		return found == nil
	})
	return found
}

// byteAt returns the offset of a zero-based line and column of content, or the length of
// content for a line past its end.
func byteAt(content []byte, line, column int) int {
	at, seen := 0, 0
	for seen < line && at < len(content) {
		if content[at] == '\n' {
			seen++
		}
		at++
	}
	return min(at+column, len(content))
}
