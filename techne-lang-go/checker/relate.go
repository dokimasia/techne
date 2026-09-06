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
	"go.dokimi.dev/techne/core/trust"
	"golang.org/x/tools/go/packages"
)

// Relate reports how a declaration connects to the rest.
//
// # Every direction is computed from the same bound program
//
// A use is an identifier the type checker mapped onto this object, which
// is what makes the answer a binding rather than a name match: two
// packages each declaring Store have two objects, and a use resolves to
// exactly one of them.
//
// Imports are the exception and are declined. They are written in the
// source rather than resolved from it, so the parser reads them without
// loading anything, and answering them here would be the same fact at a
// thousand times the price.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	if !serves(kind) {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: a type checker reads %s no better than a parser does",
			engine.ErrDecline, kind)
	}

	v, err := e.current(ctx)
	if err != nil {
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	subject, pkg, ambiguous := e.object(v, req, of)
	if ambiguous != nil {
		return engine.Result[sema.Relation]{}, ambiguous
	}
	if subject == nil {
		// Read the workspace and found no such declaration. An empty
		// answer would be a claim that nothing relates to it.
		return engine.Result[sema.Relation]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	var out []sema.Relation
	switch kind {
	case sema.References, sema.ReferencedBy:
		out = e.referring(v, subject, kind)
	case sema.Calls:
		out = e.calling(v, subject, pkg)
	case sema.CalledBy:
		out = e.callers(v, subject)
	case sema.Implements, sema.ImplementedBy:
		out = e.implementing(v, subject, kind)
	case sema.Embeds, sema.EmbeddedBy:
		out = e.incorporating(v, subject, kind)
	}

	slices.SortFunc(out, order)
	reaches, caveats := e.bound(v)
	return engine.Result[sema.Relation]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// serves reports whether a direction is one a type checker answers
// better than a parser.
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

// object is the declaration an identity names.
//
// By identity first, and by name where the kinds disagree: two engines
// answering about one declaration need not call it the same thing, and
// an identity differing in that field alone still names the same object.
//
// An identity is a language, a unit, a name and a kind, and a package
// declaring an interface method beside the method implementing it
// satisfies one twice. Two are reported rather than picked between: the
// map they are read out of has no order, so picking would answer a
// different question on different runs.
func (e *Engine) object(
	v *view,
	req engine.Request,
	of sema.ID,
) (types.Object, *packages.Package, error) {
	var exact, byName []types.Object
	var at, where []*packages.Package

	for _, pkg := range v.held() {
		for ident, held := range pkg.TypesInfo.Defs {
			if held == nil || ident.Name != of.Name() || !e.within(v, req, held) {
				continue
			}
			if found, ok := e.symbolFor(v, held); ok && found.ID == of {
				exact, at = append(exact, held), append(at, pkg)
				continue
			}
			byName, where = append(byName, held), append(where, pkg)
		}
	}

	if len(exact) > 0 {
		byName, where = exact, at
	}
	switch len(byName) {
	case 1:
		return byName[0], where[0], nil
	case 0:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf(
			"%w: %q names %d declarations: %s — narrow it with a scope",
			engine.ErrRefuse, of.Name(), len(byName), strings.Join(e.sites(v, byName), ", "))
	}
}

// sites is where several declarations sharing one identity were found,
// sorted so a refusal reads the same way twice.
func (e *Engine) sites(v *view, held []types.Object) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		at := v.fset.Position(one.Pos())
		out = append(out, fmt.Sprintf("%s:%d", e.pathOf(at.Filename), at.Line))
	}
	slices.Sort(out)
	return out
}

// within reports whether a declaration is inside the scope the request
// named.
//
// A request naming the workspace takes everything. One naming a file or
// a directory narrows to it, which is what keeps two packages declaring
// Store from answering each other's question.
func (e *Engine) within(v *view, req engine.Request, of types.Object) bool {
	if req.Scope == "" || req.Scope == engine.Root {
		return true
	}
	at := e.pathOf(v.fset.Position(of.Pos()).Filename)
	scope := string(req.Scope)
	return string(at) == scope || strings.HasPrefix(string(at), scope+"/")
}

// referring is every place the program names this declaration.
func (e *Engine) referring(
	v *view,
	subject types.Object,
	kind sema.RelationKind,
) []sema.Relation {
	var out []sema.Relation
	for _, pkg := range v.held() {
		files := read(pkg)
		for named, held := range pkg.TypesInfo.Uses {
			if held != subject {
				continue
			}
			at, via := e.sited(v, files, named.Pos(), len(named.Name))
			if !inside(at.Path) {
				continue
			}
			// The declaration the use is written inside, which is what a
			// caller reading who-uses-this wants rather than a line
			// number on its own.
			to, ok := e.enclosing(v, pkg, named.Pos())
			if !ok {
				continue
			}
			out = append(out, sema.Relation{Kind: kind, To: to, At: at, Via: via})
		}
	}
	return out
}

// callers is every function whose body calls this one.
func (e *Engine) callers(v *view, subject types.Object) []sema.Relation {
	if _, isFunc := subject.(*types.Func); !isFunc {
		return nil
	}
	var out []sema.Relation
	for _, pkg := range v.held() {
		files := read(pkg)
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, is := n.(*ast.CallExpr)
				if !is || pkg.TypesInfo.Uses[named(call.Fun)] != subject {
					return true
				}
				at, via := e.sited(v, files, call.Pos(), 0)
				if to, ok := e.enclosing(v, pkg, call.Pos()); ok && inside(at.Path) {
					out = append(out, sema.Relation{
						Kind: sema.CalledBy, To: to, At: at, Via: via,
					})
				}
				return true
			})
		}
	}
	return out
}

// calling is every function this one's body calls.
func (e *Engine) calling(
	v *view,
	subject types.Object,
	pkg *packages.Package,
) []sema.Relation {
	body := bodyOf(pkg, subject)
	if body == nil {
		return nil
	}
	files := read(pkg)

	seen := map[types.Object]bool{}
	var out []sema.Relation
	ast.Inspect(body, func(n ast.Node) bool {
		call, is := n.(*ast.CallExpr)
		if !is {
			return true
		}
		held := pkg.TypesInfo.Uses[named(call.Fun)]
		if _, isFunc := held.(*types.Func); !isFunc || seen[held] {
			return true
		}
		seen[held] = true
		if to, ok := e.symbolFor(v, held); ok {
			at, via := e.sited(v, files, call.Pos(), 0)
			out = append(out, sema.Relation{Kind: sema.Calls, To: to, At: at, Via: via})
		}
		return true
	})
	return out
}

// implementing is what satisfies an interface, or what a type satisfies.
//
// Both directions come from the same question asked the other way round,
// which is what a structural type system makes cheap: satisfying an
// interface is a property of the two types and of nothing else, so there
// is no declaration to look up and no list to keep.
func (e *Engine) implementing(
	v *view,
	subject types.Object,
	kind sema.RelationKind,
) []sema.Relation {
	held, ok := subject.(*types.TypeName)
	if !ok || held.Type() == nil {
		return nil
	}

	var out []sema.Relation
	for _, pkg := range v.held() {
		files := read(pkg)
		for _, of := range pkg.Types.Scope().Names() {
			other, is := pkg.Types.Scope().Lookup(of).(*types.TypeName)
			if !is || other == held || other.Type() == nil {
				continue
			}
			if !satisfies(held.Type(), other.Type(), kind) {
				continue
			}
			to, made := e.symbolFor(v, other)
			if !made || !inside(to.Span.Path) {
				continue
			}
			at, via := e.sited(v, files, other.Pos(), len(other.Name()))
			out = append(out, sema.Relation{Kind: kind, To: to, At: at, Via: via})
		}
	}
	return out
}

// satisfies reports whether one type implements the other, in the
// direction asked for.
//
// The pointer type is tried as well as the value: a method set declared
// on *Store satisfies an interface that a Store does not, and the
// declaration a caller asked about is the type either way.
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
		// An interface embedding another is what Embeds answers. Every
		// interface satisfies every interface it contains, and reporting
		// that as an implementation buries the types that do the work.
		return false
	}
	return types.Implements(held, face) || types.Implements(types.NewPointer(held), face)
}

// incorporating is what a type takes from, or what takes from it.
//
// An anonymous field in a struct and an embedded interface are the same
// idea and the only thing Go spells this way, so both are read from the
// declaration's own shape rather than from the method set: a type whose
// methods happen to match is not one that embeds it.
func (e *Engine) incorporating(
	v *view,
	subject types.Object,
	kind sema.RelationKind,
) []sema.Relation {
	held, ok := subject.(*types.TypeName)
	if !ok || held.Type() == nil {
		return nil
	}

	if kind == sema.Embeds {
		return e.embedded(v, held)
	}

	var out []sema.Relation
	for _, pkg := range v.held() {
		files := read(pkg)
		for _, of := range pkg.Types.Scope().Names() {
			other, is := pkg.Types.Scope().Lookup(of).(*types.TypeName)
			if !is || other == held {
				continue
			}
			if !slices.Contains(embeds(other), held) {
				continue
			}
			to, made := e.symbolFor(v, other)
			if !made || !inside(to.Span.Path) {
				continue
			}
			at, via := e.sited(v, files, other.Pos(), len(other.Name()))
			out = append(out, sema.Relation{Kind: sema.EmbeddedBy, To: to, At: at, Via: via})
		}
	}
	return out
}

// embedded is what a type takes from.
func (e *Engine) embedded(v *view, held *types.TypeName) []sema.Relation {
	var out []sema.Relation
	for _, into := range embeds(held) {
		to, made := e.symbolFor(v, into)
		if !made || !inside(to.Span.Path) {
			continue
		}
		out = append(out, sema.Relation{Kind: sema.Embeds, To: to, At: to.Span})
	}
	return out
}

// embeds is the named types a declaration writes as anonymous members.
func embeds(of *types.TypeName) []*types.TypeName {
	var out []*types.TypeName
	switch held := of.Type().Underlying().(type) {
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

// nameOf is the declaration a type refers to, through a pointer where
// one was written.
func nameOf(held types.Type) *types.TypeName {
	if pointer, is := held.(*types.Pointer); is {
		held = pointer.Elem()
	}
	if declared, is := held.(*types.Named); is {
		return declared.Obj()
	}
	return nil
}

// named is the identifier a call expression names, through a selector
// where one was written.
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

// bodyOf is the syntax of a function's body.
func bodyOf(pkg *packages.Package, of types.Object) *ast.BlockStmt {
	for _, file := range pkg.Syntax {
		for _, held := range file.Decls {
			declared, is := held.(*ast.FuncDecl)
			if is && pkg.TypesInfo.Defs[declared.Name] == of {
				return declared.Body
			}
		}
	}
	return nil
}

// order sorts edges by where they were written, so an answer reads in
// file order and does not wander between calls.
func order(a, b sema.Relation) int {
	if by := strings.Compare(string(a.At.Path), string(b.At.Path)); by != 0 {
		return by
	}
	return a.At.Start.Offset - b.At.Start.Offset
}
