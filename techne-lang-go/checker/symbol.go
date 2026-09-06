// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"golang.org/x/tools/go/packages"
)

// symbolFor turns a type checker's object into one declaration.
//
// The identity is built the same way every other engine here builds
// one — the language, the unit, the bare name and the kind — because a
// caller that resolved a declaration with one engine and asked about it
// with another has to be talking about the same thing. A method keeps
// its bare name for the same reason: gopls reports (*Store).Get and the
// parser reports Get, and the identity that has to match both is the
// one without the receiver.
func (e *Engine) symbolFor(v *view, of types.Object) (sema.Symbol, bool) {
	if of == nil || !of.Pos().IsValid() {
		return sema.Symbol{}, false
	}
	kind, classified := kindOf(of)
	if !classified {
		// A builtin, a label, the untyped nil. [sema.Kinds] leaves the
		// kind for a declaration nobody classified out of the set an
		// answer may carry, so an item holding it does not validate
		// against the shape a tool declares. Dropped rather than emitted
		// under a kind that is not one.
		return sema.Symbol{}, false
	}
	at := v.fset.Position(of.Pos())
	p := e.pathOf(at.Filename)
	unit := source.Path(e.declared.Namespace(string(p)))

	span := source.Span{
		Path:  p,
		Start: source.Position{Offset: at.Offset, Line: at.Line - 1, Column: at.Column - 1},
	}
	span.End = source.Position{
		Offset: at.Offset + len(of.Name()),
		Line:   at.Line - 1,
		Column: at.Column - 1 + len(of.Name()),
	}

	return sema.Symbol{
		ID:         sema.NewID(e.declared.Language, unit, of.Name(), kind),
		Name:       of.Name(),
		Kind:       kind,
		Language:   e.declared.Language,
		Span:       span,
		Visibility: e.declared.Visibility(of.Name()),
		Signature:  signature(of),
	}, true
}

// kindOf is what a type checker's object is, in this vocabulary, and
// reports whether it is one at all.
//
// The mapping is over what the object is rather than how it is written:
// a function with a receiver is a method, a constant is not a variable,
// and a named type is whatever it names underneath — a struct, an
// interface, or a type of its own.
//
// A builtin and the untyped nil are neither. They are objects the
// checker hands back and are not declarations anything navigates to, so
// they are reported as not classified rather than under a kind an answer
// may not carry.
func kindOf(of types.Object) (sema.Kind, bool) {
	switch held := of.(type) {
	case *types.Func:
		if held.Signature() != nil && held.Signature().Recv() != nil {
			return sema.KindMethod, true
		}
		return sema.KindFunction, true
	case *types.TypeName:
		return underlying(held), true
	case *types.Const:
		return sema.KindConstant, true
	case *types.Var:
		if held.IsField() {
			return sema.KindField, true
		}
		return sema.KindVariable, true
	case *types.PkgName:
		return sema.KindModule, true
	case *types.Label:
		return sema.KindLabel, true
	}
	return sema.KindUnknown, false
}

// underlying is what a type declaration declares, read from what it is
// underneath.
func underlying(of *types.TypeName) sema.Kind {
	if of.Type() == nil {
		return sema.KindType
	}
	switch of.Type().Underlying().(type) {
	case *types.Struct:
		return sema.KindStruct
	case *types.Interface:
		return sema.KindInterface
	}
	return sema.KindType
}

// signature is the declaration without its body, as the type checker
// writes it.
//
// A caller reading an answer wants what it takes and returns. The type
// checker's own rendering is the one that agrees with what the compiler
// bound, which a rendering built from the syntax would not for a type
// written in another package.
//
// Written relative to the declaring package, so a declaration reads as
// it was written rather than qualified with the name of the package it
// is already in.
func signature(of types.Object) string {
	held := types.ObjectString(of, types.RelativeTo(of.Pkg()))
	if at := strings.IndexByte(held, '\n'); at >= 0 {
		held = held[:at]
	}
	return held
}

// sited is where an edge was written, and the source line it was
// written on.
//
// The line comes back with it because a caller asking who calls this
// wants to read the call, and fetching each one costs a turn per site.
func (e *Engine) sited(v *view, files map[string][]byte, at token.Pos, width int) (source.Span, string) {
	if !at.IsValid() {
		return source.Span{}, ""
	}
	held := v.fset.Position(at)
	p := e.pathOf(held.Filename)
	span := source.Span{
		Path:  p,
		Start: source.Position{Offset: held.Offset, Line: held.Line - 1, Column: held.Column - 1},
		End: source.Position{
			Offset: held.Offset + width,
			Line:   held.Line - 1,
			Column: held.Column - 1 + width,
		},
	}
	return span, line(files[held.Filename], held.Offset)
}

// line is the source line an offset sits on, trimmed.
func line(content []byte, at int) string {
	if at < 0 || at > len(content) {
		return ""
	}
	from := at
	for from > 0 && content[from-1] != '\n' {
		from--
	}
	to := at
	for to < len(content) && content[to] != '\n' {
		to++
	}
	return strings.TrimSpace(string(content[from:to]))
}

// enclosing is the declaration a position was written inside, which is
// what a caller reading who-uses-this wants beside each site.
//
// The innermost one. A use in a struct's field belongs to the field
// rather than to the struct: told only the type, a caller reading
// twenty-six uses of a type sees six of them collapse onto the same
// name and cannot tell which member each was. Compared against gopls
// over one module, that was the only thing the two answers disagreed
// about.
//
// A function's own insides are not one of them. A use written in a
// signature or a body belongs to the function, and a local variable is
// not somewhere a caller navigates to.
func (e *Engine) enclosing(v *view, pkg *packages.Package, at token.Pos) (sema.Symbol, bool) {
	var found *ast.Ident
	for _, file := range pkg.Syntax {
		if file.Pos() > at || file.End() < at {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil || n.Pos() > at || n.End() < at {
				return false
			}
			if held, is := n.(*ast.FuncDecl); is {
				// Nothing inside a body is a declaration a caller
				// navigates to. A use written in one belongs to the
				// function, which is how anyone reading the file would
				// describe it.
				found = held.Name
				return false
			}
			if named := declares(n, at); named != nil {
				found = named
			}
			return true
		})
	}
	if found == nil {
		return sema.Symbol{}, false
	}
	return e.symbolFor(v, pkg.TypesInfo.Defs[found])
}

// declares is the name a node declares, and nil for a node that
// declares nothing.
//
// A field list is the one shape that is two things: a struct's members
// and an interface's methods are declarations, and a function's
// parameters and results are not. They are told apart by where the
// position sits — inside the type's own braces or inside the
// signature's brackets — which is what [Engine.enclosing] walks into.
func declares(n ast.Node, at token.Pos) *ast.Ident {
	switch held := n.(type) {
	case *ast.TypeSpec:
		return held.Name
	case *ast.ValueSpec:
		return first(held.Names)
	case *ast.StructType:
		return member(held.Fields, at)
	case *ast.InterfaceType:
		return member(held.Methods, at)
	}
	return nil
}

// member is the field or method a position falls in.
func member(fields *ast.FieldList, at token.Pos) *ast.Ident {
	if fields == nil {
		return nil
	}
	for _, field := range fields.List {
		if field.Pos() <= at && at <= field.End() {
			return first(field.Names)
		}
	}
	return nil
}

// first is the first name a declaration writes, and nil where it writes
// none: an anonymous field declares a name the language derives rather
// than one it writes.
func first(names []*ast.Ident) *ast.Ident {
	if len(names) == 0 {
		return nil
	}
	return names[0]
}

// read returns the files a package was compiled from, so a site can
// carry the line it was written on.
//
// Read once per answer rather than once per site: a declaration used
// thirty times in one file is one file.
func read(pkg *packages.Package) map[string][]byte {
	out := map[string][]byte{}
	for _, name := range pkg.CompiledGoFiles {
		if content, err := os.ReadFile(name); err == nil {
			out[name] = content
		}
	}
	return out
}
