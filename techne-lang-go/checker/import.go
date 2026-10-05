// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/packages"
)

// importers returns the import lines of the workspace that import the package that req and of
// name, by the rule of [Engine.imported]: one relation of [sema.ImportedBy] for each line, from
// the importing file, at the path of the import. It reads each source file of the view as
// [Engine.importsIn] does. A line of a test file counts when req includes tests. A file of any
// module can import the package, so the answer is as complete as the load of the whole
// workspace.
func (e *Engine) importers(v *view, req engine.Request, of sema.ID) (engine.Result[sema.Relation], error) {
	target, err := e.imported(v, req, of)
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}
	var out []sema.Relation
	for _, pkg := range v.held() {
		for _, file := range pkg.GoFiles {
			if req.Tests || !e.declared.IsTest(string(e.pathOf(file))) {
				out = append(out, e.importsIn(file, target)...)
			}
		}
	}
	slices.SortFunc(out, order)
	covered, missing := v.partial(engine.Root)
	return engine.Result[sema.Relation]{Items: out, Completeness: covered, Caveats: missing}, nil
}

// importsIn returns a relation of [sema.ImportedBy] for each import of the Go file at the
// absolute path full whose path is target. It parses the imports of the file as it is on disk,
// so a position is a position of that file. The syntax of the view of a file that imports C is
// the file that cgo generates in its place. Its line directives give the path of the original,
// and it imports unsafe in place of C. A file whose imports do not parse returns the imports
// before the error, and a file that cannot be read returns none.
func (e *Engine) importsIn(full, target string) []sema.Relation {
	content, _ := os.ReadFile(full)
	fset := token.NewFileSet()
	parsed, _ := parser.ParseFile(fset, full, content, parser.ImportsOnly)
	p := e.pathOf(full)
	var out []sema.Relation
	for _, spec := range parsed.Imports {
		if written, _ := strconv.Unquote(spec.Path.Value); written != target {
			continue
		}
		at := fset.PositionFor(spec.Path.Pos(), false)
		start := source.Position{Offset: at.Offset, Line: at.Line - 1, Column: at.Column - 1}
		end := start
		end.Offset, end.Column = start.Offset+len(spec.Path.Value), start.Column+len(spec.Path.Value)
		out = append(out, sema.Relation{
			Kind: sema.ImportedBy,
			To:   sema.File(e.declared.Language, p),
			At:   source.Span{Path: p, Start: start, End: end},
			Via:  strings.TrimSpace(lang.LineAt(content, at.Offset)),
		})
	}
	return out
}

// imported returns the import path of the package that an imported-by request names, by the
// first of these rules that applies:
//
//   - The qualified name in of is an import path that a package of the workspace has or that a
//     file of the workspace imports, such as go.thesmos.sh/core/clock or fmt.
//   - One package of the workspace under the scope of req has the qualified name in of as its
//     name, such as clock. Two or more such packages are refused with their import paths.
//   - A package of the workspace is in the directory of the scope, or in the directory of the
//     file that the scope names, as the package of a declaration such as Clock is. The view
//     lists a package before its external test package, so the package itself is the first.
//
// A command and an external test package are packages of the workspace too. No file can import
// either, so the answer about one is empty and total. imported declines a request that matches
// none of the rules.
func (e *Engine) imported(v *view, req engine.Request, of sema.ID) (string, error) {
	name, scope := of.Name(), scoped(req.Scope)
	paths := map[string]bool{}
	for _, pkg := range v.held() {
		paths[pkg.PkgPath] = true
		for imported := range pkg.Imports {
			paths[imported] = true
		}
	}
	if paths[name] {
		return name, nil
	}

	var named []string
	for _, pkg := range v.held() {
		dir, known := e.dirOf(pkg)
		if known && pkg.Name == name && lang.Within(dir, scope) && !slices.Contains(named, pkg.PkgPath) {
			named = append(named, pkg.PkgPath)
		}
	}
	slices.Sort(named)
	switch len(named) {
	case 0:
	case 1:
		return named[0], nil
	default:
		return "", fmt.Errorf("%w: checker: %s names %d packages under %s: %s. Narrow the scope to one of them",
			engine.ErrRefuse, name, len(named), scope, strings.Join(named, ", "))
	}

	home := scope
	if path.Ext(string(scope)) == ".go" {
		home = source.Path(path.Dir(string(scope)))
	}
	for _, pkg := range v.held() {
		if dir, known := e.dirOf(pkg); known && dir == home {
			return pkg.PkgPath, nil
		}
	}
	return "", fmt.Errorf("%w: checker: no package of the workspace is %s, and %s contains none",
		engine.ErrDecline, name, home)
}

// dirOf returns the workspace path of the directory of pkg, and reports whether pkg has a Go
// file to read it from.
func (e *Engine) dirOf(pkg *packages.Package) (source.Path, bool) {
	if len(pkg.GoFiles) == 0 {
		return "", false
	}
	return e.pathOf(filepath.Dir(pkg.GoFiles[0])), true
}
