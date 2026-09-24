// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/packages"
)

// Relate returns the relations of one kind from the declaration that of identifies, sorted by
// the path and the offset of their sites.
//
// The type checker binds every use to one declaration, so a use of Store in a package that
// declares Store is not a use of the Store of another package. Each kind reads:
//
//   - [sema.References] and [sema.ReferencedBy]: the uses of the declaration, each with the
//     declaration that contains it.
//   - [sema.CalledBy]: the calls of a function, each with the function that makes it.
//   - [sema.Calls]: the functions that the body of a function calls, once each.
//   - [sema.Implements] and [sema.ImplementedBy]: the interfaces that a type satisfies through
//     its value or its pointer, and the types that satisfy an interface.
//   - [sema.Embeds] and [sema.EmbeddedBy]: the types that a struct or an interface embeds, and
//     the types that embed it.
//
// Relate declines the imports and their inverse, which the parser reads from the source. It
// returns a skipped result for a scope without a Go file. It declines an ID that no
// declaration in the scope has, and refuses an ID that two or more declarations have.
//
// An error of the program of the declaration lowers the answer when it is on a line that
// writes the name of the declaration outside every site of the answer.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	if !serves(kind) {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: checker: the parser reads %s from the source", engine.ErrDecline, kind)
	}
	scope := scoped(req.Scope)
	w, err := e.walk()
	if err != nil {
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !w.claims(scope) {
		return engine.Result[sema.Relation]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}
	v, err := e.current(ctx, w)
	if err != nil {
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	subject, pkg, err := e.object(v, req, of)
	switch {
	case err != nil:
		return engine.Result[sema.Relation]{}, err
	case subject == nil:
		covered, missing := v.partial(scope)
		reason := ""
		if covered != trust.ScopeTotal {
			reason = ", and " + missing[0].Note
		}
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: checker: no declaration in %s matches %s%s",
			engine.ErrDecline, scope, of, reason)
	}

	files := sources{}
	var out []sema.Relation
	switch kind {
	case sema.References, sema.ReferencedBy:
		out = e.referring(v, files, subject, kind)
	case sema.Calls:
		out = e.calling(v, files, subject, pkg)
	case sema.CalledBy:
		out = e.callers(v, files, subject)
	case sema.Implements, sema.ImplementedBy:
		out = e.implementing(v, files, subject, kind)
	case sema.Embeds, sema.EmbeddedBy:
		out = e.incorporating(v, files, subject, kind)
	}
	slices.SortFunc(out, order)

	declared, _ := e.symbolFor(v, subject)
	sites := []source.Span{declared.Span}
	for _, one := range out {
		sites = append(sites, one.At)
	}
	tier, caveats := e.lowered(v, declared.Span.Path, lang.Writing(subject.Name(), lang.Spanned(sites...)))
	covered, missing := v.partial(scope)
	return engine.Result[sema.Relation]{
		Items:        out,
		Completeness: covered,
		Lowered:      tier,
		Caveats:      slices.Concat([]trust.Caveat{dynamic}, caveats, missing),
	}, nil
}

// serves reports whether the type checker returns the relations of kind. The parser reads the
// imports from the source.
func serves(kind sema.RelationKind) bool {
	switch kind {
	case sema.References, sema.ReferencedBy,
		sema.Calls, sema.CalledBy,
		sema.Implements, sema.ImplementedBy,
		sema.Embeds, sema.EmbeddedBy:
		return true
	}
	return false
}

// scoped returns the scope of a request, with the empty scope as [engine.Root].
func scoped(scope source.Path) source.Path {
	if scope == "" {
		return engine.Root
	}
	return scope
}

// object returns the declaration in the scope of req that of identifies, and the package that
// declares it. It returns no declaration for none, and [engine.ErrRefuse] with their sites for
// two or more.
//
// A declaration with the ID matches. Without one, a declaration of the unit and the qualified
// name of the ID matches, whatever its kind, because two engines can classify one declaration
// under different kinds. A declaration in a test file matches only when req includes tests.
func (e *Engine) object(
	v *view,
	req engine.Request,
	of sema.ID,
) (types.Object, *packages.Package, error) {
	scope := scoped(req.Scope)
	var exact, alike []types.Object
	var at, where []*packages.Package
	for _, pkg := range v.held() {
		for ident, held := range pkg.TypesInfo.Defs {
			if held == nil || ident.Name != of.Base() {
				continue
			}
			p := e.pathOf(v.fset.Position(held.Pos()).Filename)
			if !lang.Within(p, scope) || !req.Tests && e.declared.IsTest(string(p)) {
				continue
			}
			found, declares := e.symbolFor(v, held)
			switch {
			case !declares:
			case found.ID == of:
				exact, at = append(exact, held), append(at, pkg)
			case e.kinded(found, of):
				alike, where = append(alike, held), append(where, pkg)
			}
		}
	}

	if len(exact) > 0 {
		alike, where = exact, at
	}
	switch len(alike) {
	case 0:
		return nil, nil, nil
	case 1:
		return alike[0], where[0], nil
	}
	return nil, nil, fmt.Errorf("%w: checker: %s names %d declarations: %s. Narrow the scope to one of them",
		engine.ErrRefuse, of.Name(), len(alike), strings.Join(e.sites(v, alike), ", "))
}

// kinded reports whether of identifies a declaration of the unit and the qualified name of
// found, of any kind of [sema.Kinds].
func (e *Engine) kinded(found sema.Symbol, of sema.ID) bool {
	unit := source.Path(e.declared.Namespace(string(found.Span.Path)))
	return slices.ContainsFunc(sema.Kinds(), func(kind sema.Kind) bool {
		return sema.NewID(e.declared.Language, unit, found.ID.Name(), kind) == of
	})
}

// sites returns the path and the line of each object of declared, sorted.
func (e *Engine) sites(v *view, declared []types.Object) []string {
	out := make([]string, 0, len(declared))
	for _, one := range declared {
		at := v.fset.Position(one.Pos())
		out = append(out, fmt.Sprintf("%s:%d", e.pathOf(at.Filename), at.Line))
	}
	slices.Sort(out)
	return out
}

// referring returns the uses of subject in the workspace, each with the declaration that
// contains it.
func (e *Engine) referring(
	v *view,
	files sources,
	subject types.Object,
	kind sema.RelationKind,
) []sema.Relation {
	var out []sema.Relation
	for _, pkg := range v.held() {
		for named, used := range pkg.TypesInfo.Uses {
			if !v.same(used, subject) {
				continue
			}
			at, via := e.sited(v, files, named.Pos(), len(named.Name))
			if !lang.Within(at.Path, engine.Root) {
				continue
			}
			if to, known := e.enclosing(v, pkg, named.Pos()); known {
				out = append(out, sema.Relation{Kind: kind, To: to, At: at, Via: via})
			}
		}
	}
	return out
}

// callers returns the calls of the function subject in the workspace, each with the function
// that makes it.
func (e *Engine) callers(v *view, files sources, subject types.Object) []sema.Relation {
	if _, isFunc := subject.(*types.Func); !isFunc {
		return nil
	}
	var out []sema.Relation
	for _, pkg := range v.held() {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, is := n.(*ast.CallExpr)
				if !is || !v.same(pkg.TypesInfo.Uses[named(call.Fun)], subject) {
					return true
				}
				at, via := e.sited(v, files, call.Pos(), 0)
				if to, known := e.enclosing(v, pkg, call.Pos()); known && lang.Within(at.Path, engine.Root) {
					out = append(out, sema.Relation{Kind: sema.CalledBy, To: to, At: at, Via: via})
				}
				return true
			})
		}
	}
	return out
}

// calling returns the functions that the body of the function subject calls, once each, at
// the site of the first call. A call of an instance of a generic function is a call of the
// generic function.
func (e *Engine) calling(
	v *view,
	files sources,
	subject types.Object,
	pkg *packages.Package,
) []sema.Relation {
	body := bodyOf(pkg, subject)
	if body == nil {
		return nil
	}
	seen := map[*types.Func]bool{}
	var out []sema.Relation
	ast.Inspect(body, func(n ast.Node) bool {
		call, is := n.(*ast.CallExpr)
		if !is {
			return true
		}
		held, isFunc := pkg.TypesInfo.Uses[named(call.Fun)].(*types.Func)
		if !isFunc || seen[held.Origin()] {
			return true
		}
		seen[held.Origin()] = true
		if to, declares := e.symbolFor(v, held.Origin()); declares {
			at, via := e.sited(v, files, call.Pos(), 0)
			out = append(out, sema.Relation{Kind: sema.Calls, To: to, At: at, Via: via})
		}
		return true
	})
	return out
}

// implementing returns the interfaces that the type subject satisfies, for [sema.Implements],
// or the types that satisfy the interface subject, for [sema.ImplementedBy]. It compares the
// types of each package with the declaration of subject in the types of that package, by
// [counterpart], so a test package compares with the test variant of the package of subject.
func (e *Engine) implementing(
	v *view,
	files sources,
	subject types.Object,
	kind sema.RelationKind,
) []sema.Relation {
	held, is := subject.(*types.TypeName)
	if !is || held.Type() == nil {
		return nil
	}
	var out []sema.Relation
	for _, pkg := range v.held() {
		here := counterpart(v, pkg, held)
		for _, name := range pkg.Types.Scope().Names() {
			other, isType := pkg.Types.Scope().Lookup(name).(*types.TypeName)
			if !isType || v.same(other, held) || other.Type() == nil || !satisfies(here.Type(), other.Type(), kind) {
				continue
			}
			to, declares := e.symbolFor(v, other)
			if !declares || !lang.Within(to.Span.Path, engine.Root) {
				continue
			}
			at, via := e.sited(v, files, other.Pos(), len(other.Name()))
			out = append(out, sema.Relation{Kind: kind, To: to, At: at, Via: via})
		}
	}
	return out
}

// counterpart returns the declaration of the package-level type subject in the types of pkg:
// the type in the package of subject as pkg is or imports it, which can be a test variant. It
// returns subject when pkg neither is nor imports that package.
func counterpart(v *view, pkg *packages.Package, subject *types.TypeName) *types.TypeName {
	if subject.Pkg() == nil || subject.Parent() != subject.Pkg().Scope() {
		return subject
	}
	scopes := []*types.Package{pkg.Types}
	scopes = append(scopes, pkg.Types.Imports()...)
	for _, one := range scopes {
		if one.Path() != subject.Pkg().Path() {
			continue
		}
		if found, is := one.Scope().Lookup(subject.Name()).(*types.TypeName); is && v.same(found, subject) {
			return found
		}
	}
	return subject
}

// satisfies reports whether subject implements other, for [sema.Implements], or other implements
// subject, for [sema.ImplementedBy]. A type implements an interface through its value or its
// pointer. An interface that embeds another is a relation of [sema.Embeds], and an empty
// interface is satisfied by every type, so neither counts.
func satisfies(subject, other types.Type, kind sema.RelationKind) bool {
	want, held := subject, other
	if kind == sema.Implements {
		want, held = other, subject
	}
	face, is := want.Underlying().(*types.Interface)
	if !is || face.Empty() {
		return false
	}
	if _, isFace := held.Underlying().(*types.Interface); isFace {
		return false
	}
	return types.Implements(held, face) || types.Implements(types.NewPointer(held), face)
}

// incorporating returns the types that the type subject embeds, for [sema.Embeds], or the types
// that embed it, for [sema.EmbeddedBy]. An embedded field of a struct and an embedded interface
// count. A type whose methods match another's does not.
func (e *Engine) incorporating(
	v *view,
	files sources,
	subject types.Object,
	kind sema.RelationKind,
) []sema.Relation {
	held, is := subject.(*types.TypeName)
	if !is || held.Type() == nil {
		return nil
	}
	if kind == sema.Embeds {
		return e.embedded(v, held)
	}

	var out []sema.Relation
	for _, pkg := range v.held() {
		for _, name := range pkg.Types.Scope().Names() {
			other, isType := pkg.Types.Scope().Lookup(name).(*types.TypeName)
			if !isType || v.same(other, held) || other.Type() == nil {
				continue
			}
			if !slices.ContainsFunc(embeds(other), func(one *types.TypeName) bool { return v.same(one, held) }) {
				continue
			}
			to, declares := e.symbolFor(v, other)
			if !declares || !lang.Within(to.Span.Path, engine.Root) {
				continue
			}
			at, via := e.sited(v, files, other.Pos(), len(other.Name()))
			out = append(out, sema.Relation{Kind: sema.EmbeddedBy, To: to, At: at, Via: via})
		}
	}
	return out
}

// embedded returns the types in the workspace that the type held embeds, each at its
// declaration.
func (e *Engine) embedded(v *view, held *types.TypeName) []sema.Relation {
	var out []sema.Relation
	for _, into := range embeds(held) {
		to, declares := e.symbolFor(v, into)
		if !declares || !lang.Within(to.Span.Path, engine.Root) {
			continue
		}
		out = append(out, sema.Relation{Kind: sema.Embeds, To: to, At: to.Span})
	}
	return out
}

// embeds returns the named types that the type of declared embeds: the types of the embedded
// fields of a struct, and the embedded interfaces of an interface.
func embeds(declared *types.TypeName) []*types.TypeName {
	var out []*types.TypeName
	switch held := declared.Type().Underlying().(type) {
	case *types.Struct:
		for field := range held.Fields() {
			if !field.Embedded() {
				continue
			}
			if name := nameOf(field.Type()); name != nil {
				out = append(out, name)
			}
		}
	case *types.Interface:
		for embedded := range held.EmbeddedTypes() {
			if name := nameOf(embedded); name != nil {
				out = append(out, name)
			}
		}
	}
	return out
}

// nameOf returns the declaration of the named type that held is or points to, or nil for
// another type.
func nameOf(held types.Type) *types.TypeName {
	if pointer, is := held.(*types.Pointer); is {
		held = pointer.Elem()
	}
	if declared, is := held.(*types.Named); is {
		return declared.Obj()
	}
	return nil
}

// named returns the identifier of the function that a call expression calls, through a
// selector, an instantiation and parentheses, or nil for another expression.
func named(held ast.Expr) *ast.Ident {
	switch one := held.(type) {
	case *ast.Ident:
		return one
	case *ast.SelectorExpr:
		return one.Sel
	case *ast.IndexExpr:
		return named(one.X)
	case *ast.IndexListExpr:
		return named(one.X)
	case *ast.ParenExpr:
		return named(one.X)
	}
	return nil
}

// bodyOf returns the body of the declaration of the function fn in pkg, or nil for none.
func bodyOf(pkg *packages.Package, fn types.Object) *ast.BlockStmt {
	for _, file := range pkg.Syntax {
		for _, held := range file.Decls {
			declared, is := held.(*ast.FuncDecl)
			if is && pkg.TypesInfo.Defs[declared.Name] == fn {
				return declared.Body
			}
		}
	}
	return nil
}

// order compares two relations by the path of their site, then by its offset.
func order(a, b sema.Relation) int {
	if by := strings.Compare(string(a.At.Path), string(b.At.Path)); by != 0 {
		return by
	}
	return a.At.Start.Offset - b.At.Start.Offset
}
