// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

// testSuffix ends the name of a Go test file, and the package name of an external test.
const testSuffix = "_test"

// Plan plans [edit.MoveFile] by the rules of the package comment under Moves, and declines
// every other operation, which gopls plans. Each role edits a file as follows:
//
//   - The package clause of the moved file becomes the name of the destination package, with
//     the _test suffix of an external test. A use of a declaration that remains in the source
//     package gets the qualifier of the source package. A use of a declaration of the
//     destination loses its qualifier.
//   - Another file of the source package qualifies each use of a moved declaration with the
//     destination package.
//   - A file of the destination package drops the qualifier of the source package from each use
//     of a moved declaration.
//   - Any other file qualifies each use of a moved declaration with the destination package.
//
// A move within the directory of the file moves the file alone. A file of the source package
// that the build constraints of the load exclude and that refers to a moved declaration makes
// the plan partial.
//
// Plan returns the refusals of [lang.Moving], and [engine.ErrRefuse] for a destination that is
// not a Go file, a move across the _test suffix, a directory whose name is no identifier, and a
// use that the move cannot qualify. It returns a skipped result for a file of another language.
func (e *Engine) Plan(
	ctx context.Context,
	_ engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	if op != edit.MoveFile {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: checker: the type checker plans %s and no other operation", engine.ErrDecline, edit.MoveFile)
	}
	from, to, err := e.ends(target, args)
	if err != nil || from == "" {
		return engine.Result[edit.Change]{Skipped: err == nil, Completeness: trust.ScopeTotal}, err
	}

	// A move within the directory changes no package, so it needs no load.
	fromFull, toFull := e.fullPath(from), e.fullPath(to)
	if filepath.Dir(fromFull) == filepath.Dir(toFull) {
		return engine.Result[edit.Change]{
			Items:        []edit.Change{{Kind: edit.ChangeMove, Path: from, To: to}},
			Completeness: trust.ScopeTotal,
			Caveats:      []trust.Caveat{dynamic},
		}, nil
	}
	w, err := e.walk()
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	v, err := e.current(ctx, w)
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	pkg, file := compiling(v, fromFull)
	if pkg == nil {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: checker: no loaded package compiles %s", engine.ErrDecline, from)
	}
	m, err := v.moving(pkg, file, filepath.Dir(toFull))
	if err != nil {
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: checker: %w", engine.ErrRefuse, err)
	}
	m.source, m.destination = fromFull, toFull

	// held contains each file once, and Syntax contains the syntax of the files of
	// CompiledGoFiles, in that order.
	rewrites := map[string]*rewrite{}
	var stuck []string
	for _, one := range v.held() {
		for i, f := range one.Syntax {
			if full := v.sourceOf(one, i); full != "" {
				stuck = append(stuck, m.typed(v, one, f, full, rewrites)...)
			}
		}
	}
	if len(stuck) > 0 {
		names := slices.Compact(slices.Sorted(slices.Values(stuck)))
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: after the move, packages %s and %s use these declarations of each other, and Go allows that "+
				"only for an exported name outside an external test package: %s",
			engine.ErrRefuse, m.fromName, m.toName, strings.Join(names, ", "))
	}
	// A file that the build constraints exclude is an ignored file of one held package.
	var unseen []string
	for _, one := range v.held() {
		for _, full := range one.IgnoredFiles {
			if m.untyped(full, rewrites) {
				unseen = append(unseen, string(e.pathOf(full)))
			}
		}
	}

	changes, byName, err := e.written(m, rewrites)
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	changes = append(changes, edit.Change{Kind: edit.ChangeMove, Path: from, To: to})
	covered, caveats := v.partial(engine.Root)
	caveats = append([]trust.Caveat{dynamic}, caveats...)
	if len(byName) > 0 {
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatInactiveBuild,
			Note: "the build constraints of the load exclude these files, so the plan rewrites the " +
				"uses of the moved declarations in them by name: " + strings.Join(byName, ", "),
			Paths: paths(byName),
		})
	}
	if len(unseen) > 0 {
		covered = trust.ScopePartial
		slices.Sort(unseen)
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatUnrewritten,
			Note: "these files of the source package, which the build constraints of the load " +
				"exclude, name a moved declaration that the plan does not qualify: " + strings.Join(unseen, ", "),
			Paths: paths(unseen),
		})
	}
	reaches, why := lang.Lowered(e.errors(program{pkgs: v.all()}), e.lines, m.hides, "the type checker")
	return engine.Result[edit.Change]{
		Items:        changes,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      append(caveats, why...),
	}, nil
}

// ends returns the workspace paths of the file that target names and of the destination in
// args. It returns an empty source for a file of another language, the refusals of
// [lang.Moving], and [engine.ErrRefuse] for a destination that is not a Go file or that differs
// from the source in being a test file.
func (e *Engine) ends(target edit.Target, args edit.Args) (source.Path, source.Path, error) {
	from, to, err := lang.Moving(e.root, target, args, e.declared.Extensions)
	switch {
	case err != nil || from == "":
		return "", "", err
	case !lang.Claims(string(to), e.declared.Extensions):
		return "", "", fmt.Errorf(
			"%w: %s is not a Go file, and the move keeps %s a Go file", engine.ErrRefuse, to, from)
	case e.declared.IsTest(string(from)) != e.declared.IsTest(string(to)):
		return "", "", fmt.Errorf("%w: a move keeps a test file a test file, and %s and %s differ",
			engine.ErrRefuse, from, to)
	}
	return from, to, nil
}

// compiling returns the package of v that compiles the file at full, with its syntax, or nil. The
// syntax of a file that imports C is the file that cgo generates in its place, by the rule of
// [view.sourceOf].
func compiling(v *view, full string) (*packages.Package, *ast.File) {
	for _, pkg := range v.held() {
		for i, f := range pkg.Syntax {
			if v.sourceOf(pkg, i) == full {
				return pkg, f
			}
		}
	}
	return nil, nil
}

// move is what one move changes: the two packages, the declarations that move and the
// declarations that remain in the source package.
type move struct {
	// moved and remaining are the top-level declarations of the moved file and of the other
	// files of the source package, by the position of their name. The objects of one
	// declaration in a package and its test variant share that position.
	moved, remaining map[token.Position]bool
	// movedNames and remainingNames are the names of those declarations without methods.
	movedNames, remainingNames map[string]bool
	source, destination        string
	// from and to are the name and the import path of the source and the destination package.
	fromName, fromPath string
	toName, toPath     string
}

// moving returns the move of file, a file of pkg, into the directory dir.
func (v *view) moving(pkg *packages.Package, file *ast.File, dir string) (*move, error) {
	toName, toPath, err := v.packageIn(dir)
	if err != nil {
		return nil, err
	}
	m := &move{
		fromName: pkg.Name, fromPath: pkg.PkgPath, toName: toName, toPath: toPath,
		moved: map[token.Position]bool{}, remaining: map[token.Position]bool{},
		movedNames: map[string]bool{}, remainingNames: map[string]bool{},
	}
	for i, f := range pkg.Syntax {
		// The declarations that cgo generates for no file of the package, such as the functions
		// that call C, go with the file that calls them.
		if v.sourceOf(pkg, i) == "" {
			continue
		}
		positions, names := m.moved, m.movedNames
		if f != file {
			positions, names = m.remaining, m.remainingNames
		}
		// Defs contains an object for every top-level name, the blank name included.
		for _, name := range topLevel(f) {
			positions[v.fset.Position(pkg.TypesInfo.Defs[name].Pos())] = true
			names[name.Name] = true
		}
	}
	return m, nil
}

// packageIn returns the name and the import path of the package in dir: the package of the Go
// files in dir without the suffix of an external test, and otherwise a package named for dir in
// the module that contains dir.
func (v *view) packageIn(dir string) (string, string, error) {
	var module *packages.Module
	for _, pkg := range v.all() {
		for _, full := range pkg.CompiledGoFiles {
			if filepath.Dir(full) == dir {
				return strings.TrimSuffix(pkg.Name, testSuffix), strings.TrimSuffix(pkg.PkgPath, testSuffix), nil
			}
		}
		if one := pkg.Module; one != nil && one.Dir != "" && below(dir, one.Dir) &&
			(module == nil || len(one.Dir) > len(module.Dir)) {

			module = one
		}
	}
	if module == nil {
		return "", "", fmt.Errorf("no loaded module contains %s", dir)
	}
	name := filepath.Base(dir)
	if !token.IsIdentifier(name) {
		return "", "", fmt.Errorf("the directory %s names no Go package, because %s is no identifier", dir, name)
	}
	relative, _ := filepath.Rel(module.Dir, dir)
	if relative == "." {
		return name, module.Path, nil
	}
	return name, module.Path + "/" + filepath.ToSlash(relative), nil
}

// topLevel returns the names of the top-level declarations of f other than methods.
func topLevel(f *ast.File) []*ast.Ident {
	var out []*ast.Ident
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				out = append(out, d.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					out = append(out, s.Name)
				case *ast.ValueSpec:
					out = append(out, s.Names...)
				}
			}
		}
	}
	return out
}

// role is how a move changes a file.
type role int

const (
	// moved is the file that moves.
	moved role = iota
	// behind is another file of the source package.
	behind
	// arriving is a file of the destination package.
	arriving
	// importing is any other file.
	importing
)

// roleOf returns the role of the file at full in the package with the import path pkgPath.
func (m *move) roleOf(full, pkgPath string) role {
	switch {
	case full == m.source:
		return moved
	case pkgPath == m.fromPath:
		return behind
	case pkgPath == m.toPath:
		return arriving
	}
	return importing
}

// rewrite is how a move changes one file: its edits, the imports that it adds, and whether it
// imports the destination in place of the source.
type rewrite struct {
	// alias is the name under which the file imports the destination, or empty for the name of
	// the destination package.
	alias string
	edits []edit.TextEdit
	adds  []string
	swap  bool
	// byName reports that the file has no types and the move rewrote it by name.
	byName bool
}

// edited returns the rewrite of the file at full in rewrites, which it adds when it is missing.
func edited(rewrites map[string]*rewrite, full string) *rewrite {
	if rewrites[full] == nil {
		rewrites[full] = &rewrite{}
	}
	return rewrites[full]
}

// insert returns the edit that inserts text at offset.
func insert(offset int, text string) edit.TextEdit {
	at := source.Position{Offset: offset}
	return edit.TextEdit{Span: source.Span{Start: at, End: at}, New: text}
}

// replace returns the edit that replaces the bytes from start to end with text.
func replace(start, end int, text string) edit.TextEdit {
	return edit.TextEdit{
		Span: source.Span{Start: source.Position{Offset: start}, End: source.Position{Offset: end}},
		New:  text,
	}
}

// typed adds the edits of f, the syntax of the file at full of pkg, to rewrites. It binds each
// name with the types of pkg, and places each edit in the file at full by the rule of
// [view.placed], so an edit of a file that cgo generates in place of the file is an edit of the
// file. It returns the names that the move has to qualify and cannot: an unexported name, and any
// name of an external test package, which no file imports.
func (m *move) typed(
	v *view,
	pkg *packages.Package,
	f *ast.File,
	full string,
	rewrites map[string]*rewrite,
) []string {
	r := m.roleOf(full, pkg.PkgPath)
	selected := map[*ast.Ident]*ast.SelectorExpr{}
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			selected[s.Sel] = s
		}
		return true
	})
	offset := func(p token.Pos) int {
		_, at := v.placed(p)
		return at.Offset
	}
	external := strings.HasSuffix(m.fromName, testSuffix)
	target := m.toName
	if local := importedAs(f, m.toPath, m.toName); local != "" {
		target = local
	}
	var edits []edit.TextEdit
	var adds, stuck []string
	var same []*ast.SelectorExpr
	keeps := false
	crossing := func(name *ast.Ident, qualifier, imports string) {
		if !name.IsExported() || external {
			stuck = append(stuck, name.Name)
			return
		}
		edits, adds = append(edits, insert(offset(name.Pos()), qualifier+".")), append(adds, imports)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		name, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		object := pkg.TypesInfo.Uses[name]
		if object == nil {
			return true
		}
		declared := v.fset.Position(object.Pos())
		// The name of a top-level declaration is the selector of a selector expression only
		// behind the name of an imported package.
		s, qualified := selected[name]
		switch {
		case r == moved && m.remaining[declared] && !qualified:
			crossing(name, m.fromName, m.fromPath)
		case r == moved && qualified && !external && qualifier(pkg, s) == m.toPath:
			edits = append(edits, replace(offset(s.X.Pos()), offset(s.Sel.Pos()), ""))
		case r == behind && m.moved[declared] && !qualified:
			crossing(name, target, m.toPath)
		case r == arriving && m.moved[declared] && qualified:
			edits = append(edits, replace(offset(s.X.Pos()), offset(s.Sel.Pos()), ""))
		case r == importing && m.moved[declared] && qualified && m.fromName == m.toName:
			same = append(same, s)
		case r == importing && m.moved[declared] && qualified:
			edits = append(edits, replace(offset(s.X.Pos()), offset(s.X.End()), target))
			adds = append(adds, m.toPath)
		case r == importing && qualified && qualifier(pkg, s) == m.fromPath:
			keeps = true
		}
		return true
	})
	if r == moved && m.fromName != m.toName {
		clause := m.toName
		if external {
			clause += testSuffix
		}
		edits = append(edits, replace(offset(f.Name.Pos()), offset(f.Name.End()), clause))
	}
	alias := ""
	if keeps && len(same) > 0 {
		alias = m.alias(f)
		adds = append(adds, m.toPath)
		for _, s := range same {
			edits = append(edits, replace(offset(s.X.Pos()), offset(s.X.End()), alias))
		}
	}
	swap := len(same) > 0 && !keeps

	// A godoc link names a package by the name of an import of its file, and any other package
	// by its import path.
	links := linking{from: m.fromPath, to: m.toPath}
	if slices.Contains(adds, m.fromPath) {
		links.from = m.fromName
	}
	switch {
	case alias != "":
		links.to = alias
	case swap:
		links.to = m.fromName
	case slices.Contains(adds, m.toPath) || importedAs(f, m.toPath, m.toName) != "":
		links.to = target
	}
	if !external {
		links.own = importedAs(f, m.toPath, m.toName)
	}
	edits = append(edits, m.linked(offset, f, r, links)...)
	if len(edits) == 0 && !swap {
		return stuck
	}
	one := edited(rewrites, full)
	one.edits, one.adds = append(one.edits, edits...), append(one.adds, adds...)
	one.swap = one.swap || swap
	if alias != "" {
		one.alias = alias
	}
	return stuck
}

// alias returns a name for the import of the destination in f, a file that also imports the
// source package under the same name: the name of the parent directory of the destination
// followed by the name of the destination, with a number when f already uses that name.
func (m *move) alias(f *ast.File) string {
	base := path.Base(path.Dir(m.toPath)) + m.toName
	if !token.IsIdentifier(base) {
		base = m.toName
	}
	used := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if name, ok := n.(*ast.Ident); ok {
			used[name.Name] = true
		}
		return true
	})
	out := base
	for i := 2; used[out]; i++ {
		out = fmt.Sprintf("%s%d", base, i)
	}
	return out
}

// qualifier returns the import path of the package that s qualifies with.
func qualifier(pkg *packages.Package, s *ast.SelectorExpr) string {
	x, _ := s.X.(*ast.Ident)
	if named, ok := pkg.TypesInfo.Uses[x].(*types.PkgName); ok {
		return named.Imported().Path()
	}
	return ""
}

// untyped adds the edits of the file at full, which the build constraints of the load exclude,
// to rewrites. It matches the moved declarations by name. It reports true for a file of the
// source package with an identifier that has the name of a moved declaration and is not a
// top-level name of the file, which it does not rewrite.
func (m *move) untyped(full string, rewrites map[string]*rewrite) bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, full, nil, parser.ParseComments)
	if err != nil {
		return false
	}
	dir := filepath.Dir(full)
	offset := func(p token.Pos) int { return fset.Position(p).Offset }
	if dir == filepath.Dir(m.source) && f.Name.Name == m.fromName {
		named := false
		declared := map[*ast.Ident]bool{}
		for _, one := range topLevel(f) {
			declared[one] = true
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if x, ok := n.(*ast.Ident); ok {
				named = named || m.movedNames[x.Name] && !declared[x]
			}
			return true
		})
		return named
	}
	local := importedAs(f, m.fromPath, m.fromName)
	if local == "" {
		return false
	}
	arrives := dir == filepath.Dir(m.destination) && f.Name.Name == m.toName
	target := m.toName
	if held := importedAs(f, m.toPath, m.toName); held != "" {
		target = held
	}
	var edits []edit.TextEdit
	var same []*ast.Ident
	keeps := false
	ast.Inspect(f, func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, isName := s.X.(*ast.Ident)
		switch {
		case !isName || x.Name != local:
		case !m.movedNames[s.Sel.Name]:
			keeps = true
		case arrives:
			edits = append(edits, replace(offset(x.Pos()), offset(s.Sel.Pos()), ""))
		case m.fromName == m.toName:
			same = append(same, x)
		default:
			edits = append(edits, replace(offset(x.Pos()), offset(x.End()), target))
		}
		return true
	})
	alias := ""
	if keeps && len(same) > 0 {
		alias = m.alias(f)
		for _, x := range same {
			edits = append(edits, replace(offset(x.Pos()), offset(x.End()), alias))
		}
	}
	swap := len(same) > 0 && !keeps
	if len(edits) == 0 && !swap {
		return false
	}
	one := edited(rewrites, full)
	one.edits, one.swap, one.alias, one.byName = append(one.edits, edits...), swap, alias, true
	if len(edits) > 0 && !arrives {
		one.adds = append(one.adds, m.toPath)
	}
	return false
}

// importedAs returns the name by which f imports the package with the import path imported,
// whose package clause names name, or the empty string when f does not import it or imports it
// blank or dotted.
func importedAs(f *ast.File, imported, name string) string {
	for _, spec := range f.Imports {
		if strings.Trim(spec.Path.Value, "\"`") != imported {
			continue
		}
		switch {
		case spec.Name == nil:
			return name
		case spec.Name.Name == "_" || spec.Name.Name == ".":
			return ""
		}
		return spec.Name.Name
	}
	return ""
}

// docLink matches a godoc link: a bracketed name, qualified by a package name, an import path
// or a type.
var docLink = regexp.MustCompile(`\[((?:[\pL\pN_.~-]+/)*[\pL_][\pL\pN_]*(?:\.[\pL_][\pL\pN_]*)*)\]`)

// linking is how the godoc links of one file qualify a declaration of each package: by the
// name of an import of the file, or by the import path of a package that the file does not
// import.
type linking struct {
	// from qualifies a declaration of the source package, and to a declaration of the
	// destination package.
	from, to string
	// own is the name of the import of the destination in the moved file, whose links lose
	// the qualifier, or empty.
	own string
}

// linked returns the edits that keep the godoc links of f, a file of the role r, pointing at
// the declarations that they name, with the qualifiers of links. offset returns the offset of a
// position of f in the file that the edits change.
func (m *move) linked(offset func(token.Pos) int, f *ast.File, r role, links linking) []edit.TextEdit {
	var out []edit.TextEdit
	for _, group := range f.Comments {
		for _, c := range group.List {
			start := offset(c.Pos())
			for _, at := range docLink.FindAllStringSubmatchIndex(c.Text, -1) {
				begin := start + at[2]
				if one, rewritten := m.pathLinked(c.Text[at[2]:at[3]], begin, r); rewritten {
					out = append(out, one)
					continue
				}
				first, rest, qualified := strings.Cut(c.Text[at[2]:at[3]], ".")
				next, _, _ := strings.Cut(rest, ".")
				switch {
				case r == moved && m.remainingNames[first]:
					out = append(out, insert(begin, links.from+"."))
				case r == moved && qualified && links.own != "" && first == links.own:
					out = append(out, replace(begin, begin+len(first)+1, ""))
				case r == behind && m.movedNames[first]:
					out = append(out, insert(begin, links.to+"."))
				case r == arriving && qualified && first == m.fromName && m.movedNames[next]:
					out = append(out, replace(begin, begin+len(first)+1, ""))
				case r == importing && qualified && first == m.fromName && m.movedNames[next]:
					out = append(out, replace(begin, begin+len(first), links.to))
				}
			}
		}
	}
	return out
}

// pathLinked returns the edit of a godoc link inner at the offset begin that names a moved
// declaration by the import path of the source package, and reports whether it returns one. A
// file of the destination drops the path, and any other file writes the path of the
// destination.
func (m *move) pathLinked(inner string, begin int, r role) (edit.TextEdit, bool) {
	slash := strings.LastIndex(inner, "/")
	dot := strings.Index(inner[slash+1:], ".")
	if dot < 0 {
		return edit.TextEdit{}, false
	}
	end := slash + 1 + dot
	name, _, _ := strings.Cut(inner[end+1:], ".")
	switch {
	case inner[:end] != m.fromPath || !m.movedNames[name]:
		return edit.TextEdit{}, false
	case r == arriving:
		return replace(begin, begin+end+1, ""), true
	}
	return replace(begin, begin+end, m.toPath), true
}

// hides reports whether an error on a line can hide a use of a moved declaration: the line
// names the source package or a moved declaration.
func (m *move) hides(_ source.Path, _ int, text string) bool {
	if lang.Worded(text, m.fromName) >= 0 {
		return true
	}
	for name := range m.movedNames {
		if lang.Worded(text, name) >= 0 {
			return true
		}
	}
	return false
}

// written returns the change of each file of rewrites: the lines that its edits and its
// imports change, as gofmt prints the file. It returns the workspace paths of the files that
// it rewrote by name.
func (e *Engine) written(m *move, rewrites map[string]*rewrite) ([]edit.Change, []string, error) {
	var out []edit.Change
	var byName []string
	for _, full := range slices.Sorted(maps.Keys(rewrites)) {
		one := rewrites[full]
		original, err := os.ReadFile(full)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: checker: read %s: %w", engine.ErrDecline, e.pathOf(full), err)
		}
		slices.SortStableFunc(one.edits, func(a, b edit.TextEdit) int {
			return a.Span.Start.Offset - b.Span.Start.Offset
		})
		applied, err := edit.Apply(original, one.edits)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: checker: %s: %w", engine.ErrDecline, e.pathOf(full), err)
		}
		printed, err := m.imports(full, applied, one)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: checker: %s: %w", engine.ErrDecline, e.pathOf(full), err)
		}
		p := e.pathOf(full)
		if hunked := hunks(p, original, printed); len(hunked) > 0 {
			out = append(out, edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: hunked})
		}
		if one.byName {
			byName = append(byName, string(p))
		}
	}
	return out, byName, nil
}

// imports returns content, a file that a move rewrote, with the imports that it uses: the
// source or the destination package where it names them, and the destination in place of the
// source for a swap and by the rule of [move.replaces]. It prints the file as gofmt does, with
// the line endings of content.
func (m *move) imports(full string, content []byte, r *rewrite) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, full, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if r.swap || m.replaces(f) {
		astutil.RewriteImport(fset, f, m.fromPath, m.toPath)
	}
	names := map[string]string{m.fromPath: m.fromName, m.toPath: m.toName}
	if r.alias != "" {
		names[m.toPath] = r.alias
	}
	// The deletions come first, so an import that replaces the last import of a file is not
	// left in parentheses.
	for _, one := range []string{m.fromPath, m.toPath} {
		if local := importedAs(f, one, names[one]); local != "" && !qualifies(f, local) {
			for astutil.DeleteNamedImport(fset, f, importName(f, one), one) {
			}
		}
	}
	for _, one := range slices.Compact(slices.Sorted(slices.Values(r.adds))) {
		if importedAs(f, one, names[one]) != "" {
			continue
		}
		if names[one] == path.Base(one) {
			astutil.AddImport(fset, f, one)
		} else {
			astutil.AddNamedImport(fset, f, names[one], one)
		}
	}
	var out bytes.Buffer
	if failed := format.Node(&out, fset, f); failed != nil {
		return nil, fmt.Errorf("print: %w", failed)
	}
	grouped, err := imports.Process(full, out.Bytes(), &printing)
	if err != nil {
		return nil, fmt.Errorf("group the imports: %w", err)
	}
	if bytes.Contains(content, []byte("\r\n")) {
		return bytes.ReplaceAll(grouped, []byte("\n"), []byte("\r\n")), nil
	}
	return grouped, nil
}

// replaces reports whether f, a file that a move rewrote, imports the destination in place of
// the source. f then imports the source under a name, not blank and not a dot, and no longer
// qualifies a name with it. The destination has the last element of its import path as its name,
// and f does not import it. The path of the destination then replaces the path of the source in
// the same import, on the same line. A new import would follow the import of C of a file of cgo
// on the next line, without the blank line between them. [move.imports] then deletes an import
// that f does not use and adds an import that f needs, as for any other file.
func (m *move) replaces(f *ast.File) bool {
	local := importedAs(f, m.fromPath, m.fromName)
	return local != "" && !qualifies(f, local) && m.toName == path.Base(m.toPath) &&
		importedAs(f, m.toPath, m.toName) == ""
}

// printing prints a file as goimports does, and does not add or delete an import.
var printing = imports.Options{Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: true}

// qualifies reports whether a selector of f qualifies a name with local, the name of an import.
func qualifies(f *ast.File, local string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if x, isName := s.X.(*ast.Ident); isName && x.Name == local {
				found = true
			}
		}
		return !found
	})
	return found
}

// importName returns the name that the import of the package with the import path imported
// writes in f, or the empty string for an import without one.
func importName(f *ast.File, imported string) string {
	for _, spec := range f.Imports {
		if strings.Trim(spec.Path.Value, "\"`") == imported && spec.Name != nil {
			return spec.Name.Name
		}
	}
	return ""
}

// paths returns names as workspace paths.
func paths(names []string) []source.Path {
	out := make([]source.Path, 0, len(names))
	for _, one := range names {
		out = append(out, source.Path(one))
	}
	return out
}

// scripted is the number of differences above which [hunks] replaces the whole file with one
// edit. The trace of the script takes about scripted² integers: 8 MiB at 1024.
const scripted = 1024

// hunks returns the edits that turn before into after: one edit for each run of lines that
// differ, which replaces the lines of before with the lines of after. The lines are the
// shortest edit script of Myers.
func hunks(p source.Path, before, after []byte) []edit.TextEdit {
	a, b := lined(before), lined(after)
	starts := make([]int, len(a)+1)
	for i, line := range a {
		starts[i+1] = starts[i] + len(line)
	}
	var out []edit.TextEdit
	add := func(i, j, k, l int) {
		if i == j && k == l {
			return
		}
		out = append(out, edit.TextEdit{
			Span: source.Span{
				Path:  p,
				Start: source.Position{Line: i, Offset: starts[i]},
				End:   source.Position{Line: j, Offset: starts[j]},
			},
			New: strings.Join(b[k:l], ""),
		})
	}
	i, k := 0, 0
	for _, pair := range matched(a, b) {
		add(i, pair[0], k, pair[1])
		i, k = pair[0]+1, pair[1]+1
	}
	add(i, len(a), k, len(b))
	return out
}

// lined returns the lines of content, each with its line ending.
func lined(content []byte) []string {
	var out []string
	for line := range bytes.Lines(content) {
		out = append(out, string(line))
	}
	return out
}

// matched returns the pairs of indexes of the lines that a and b have in common, in order, by
// the shortest edit script of Myers, or none when the script is longer than [scripted].
func matched(a, b []string) [][2]int {
	n, m := len(a), len(b)
	most := min(n+m, scripted)
	center := most + 1
	v := make([]int, 2*most+3)
	var trace [][]int
	found := -1
	for d := 0; d <= most && found < 0; d++ {
		trace = append(trace, slices.Clone(v[center-d:center+d+1]))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[center+k-1] < v[center+k+1]) {
				x = v[center+k+1]
			} else {
				x = v[center+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[center+k] = x
			if x >= n && y >= m {
				found = d
				break
			}
		}
	}
	if found < 0 {
		return nil
	}
	var out [][2]int
	x, y := n, m
	for d := found; d > 0; d-- {
		// prior is the furthest x of each diagonal from -d to d before step d.
		prior := trace[d]
		k := x - y
		previous := k - 1
		if k == -d || (k != d && prior[d+k-1] < prior[d+k+1]) {
			previous = k + 1
		}
		px := prior[d+previous]
		py := px - previous
		for x > px && y > py {
			out = append(out, [2]int{x - 1, y - 1})
			x, y = x-1, y-1
		}
		x, y = px, py
	}
	for x > 0 && y > 0 {
		out = append(out, [2]int{x - 1, y - 1})
		x, y = x-1, y-1
	}
	slices.Reverse(out)
	return out
}
