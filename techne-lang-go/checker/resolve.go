// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"golang.org/x/tools/go/packages"
)

// Resolve reports what the name at a position denotes.
//
// One answer or none. A name in a type-checked program denotes one
// object, and the ambiguity the port allows for is a thing weaker
// engines have: a parser matching text finds every declaration of that
// name and cannot say which one is meant.
//
// A position may carry only a line and a column, because whoever asked
// is looking at an editor. The offset is worked out here from the file,
// which is the one place this vocabulary's authoritative coordinate is
// derived rather than believed.
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	p := req.Scope
	if filepath.Ext(string(p)) != ".go" {
		return engine.Result[sema.Symbol]{}, fmt.Errorf(
			"%w: %q is not a Go file", engine.ErrDecline, p)
	}

	v, err := e.current(ctx)
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	content, err := os.ReadFile(filepath.Join(e.root, filepath.FromSlash(string(p))))
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("checker: read %s: %w", p, err)
	}
	offset := at.Offset
	if offset <= 0 && (at.Line > 0 || at.Column > 0) {
		offset = byteAt(content, at.Line, at.Column)
	}

	pkg, file, ok := e.holding(v, p)
	if !ok {
		// The workspace holds no package with this file in it, which is
		// a file outside every module rather than a broken engine.
		return engine.Result[sema.Symbol]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	named := identAt(file, v.fset, offset)
	if named == nil {
		return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
	}

	// Uses first: a name written where it is used denotes what it was
	// declared as, and that is what a caller pointing at it wants. Defs
	// covers a caller pointing at the declaration itself.
	of := pkg.TypesInfo.Uses[named]
	if of == nil {
		of = pkg.TypesInfo.Defs[named]
	}
	found, resolved := e.symbolFor(v, of)
	if !resolved {
		return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
	}
	found.Snippet = line(content, offset)

	reaches, caveats := e.bound(v)
	return engine.Result[sema.Symbol]{
		Items:        []sema.Symbol{found},
		Completeness: trust.ScopeTotal,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// holding is the package and file a path belongs to.
func (e *Engine) holding(v *view, p source.Path) (*packages.Package, *ast.File, bool) {
	full := filepath.Join(e.root, filepath.FromSlash(string(p)))
	for _, pkg := range v.held() {
		for _, file := range pkg.Syntax {
			if v.fset.Position(file.Pos()).Filename == full {
				return pkg, file, true
			}
		}
	}
	return nil, nil, false
}

// identAt is the identifier written at an offset, and nil where the
// offset is not on one.
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

// byteAt is the offset a line and column name, both counting from zero.
//
// A caller looking at an editor sends those and no offset, and only
// something holding the file can turn them into one.
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
