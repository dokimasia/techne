// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/packages"
)

// symbolFor returns the declaration of an object of the type checker, and reports whether the
// object is a declaration that an answer can contain. A builtin, the untyped nil and an object
// without a position are not.
//
// The ID has the language, the unit, the qualified name that [Engine.qualified] returns and the
// kind, as the parser and the lsp engine build it. An object with types from export data gets
// the span and the ID of its declaration from source, by [view.sourced]. An object of the
// standard library gets the span of its name in its file, which [view.expanded] names.
func (e *Engine) symbolFor(v *view, of types.Object) (sema.Symbol, bool) {
	if of == nil || !of.Pos().IsValid() {
		return sema.Symbol{}, false
	}
	of = v.sourced(of)
	kind, classified := kindOf(of)
	if !classified {
		return sema.Symbol{}, false
	}

	at := v.fset.Position(of.Pos())
	file := v.expanded(at.Filename)
	start := source.Position{Offset: at.Offset, Line: at.Line - 1, Column: at.Column - 1}
	if v.file(of.Pos()) == nil {
		start = located(file, at.Line-1, of.Name())
	}
	p := e.pathOf(file)
	unit := source.Path(e.declared.Namespace(string(p)))
	width := len(of.Name())
	return sema.Symbol{
		ID:       sema.NewID(e.declared.Language, unit, e.qualified(v, of), kind),
		Name:     of.Name(),
		Kind:     kind,
		Language: e.declared.Language,
		Span: source.Span{
			Path:  p,
			Start: start,
			End:   source.Position{Offset: start.Offset + width, Line: start.Line, Column: start.Column + width},
		},
		Visibility: e.declared.Visibility(of.Name()),
		Signature:  signature(of),
	}, true
}

// located returns the position of name on the zero-based line of the file at full, for an
// object with types from export data, whose position has a line and no column. It returns the
// start of the line when the line does not contain name as a word, and a position with the line
// alone when the file cannot be read.
func located(full string, line int, name string) source.Position {
	content, err := os.ReadFile(full)
	if err != nil {
		return source.Position{Line: line}
	}
	start := byteAt(content, line, 0)
	end := len(content)
	if at := bytes.IndexByte(content[start:], '\n'); at >= 0 {
		end = start + at
	}
	column := max(lang.Worded(string(content[start:end]), name), 0)
	return source.Position{Offset: start + column, Line: line, Column: column}
}

// same reports whether one and other are one declaration.
//
// In one program they share the position of their name. That covers a declaration and its
// counterpart in the test variant of its package, and a generic declaration and its instances.
// An object with types from export data, such as a declaration of a module that another
// program type-checks, has a position with a file and a line and no column. It is the
// declaration of that name on that line of that file.
func (v *view) same(one, other types.Object) bool {
	switch {
	case one == nil || other == nil || one.Name() != other.Name():
		return false
	case one.Pos() == other.Pos():
		return one.Pos().IsValid()
	case v.file(one.Pos()) != nil && v.file(other.Pos()) != nil:
		return false
	}
	a, b := v.fset.Position(one.Pos()), v.fset.Position(other.Pos())
	return a.IsValid() && a.Filename == b.Filename && a.Line == b.Line
}

// sourced returns the declaration from source that of names, for an object with types from
// export data: the declaration of that name on the line of its position, in a package whose
// syntax the view has. It returns of for an object with syntax, and for a file that no package
// of the view type-checks, such as a file of the standard library.
func (v *view) sourced(of types.Object) types.Object {
	at := v.fset.Position(of.Pos())
	if v.file(of.Pos()) != nil || !v.compiles(at.Filename) {
		return of
	}
	for _, pkg := range v.held() {
		if !slices.Contains(pkg.CompiledGoFiles, at.Filename) {
			continue
		}
		for ident, declared := range pkg.TypesInfo.Defs {
			if declared == nil || ident.Name != of.Name() {
				continue
			}
			if there := v.fset.Position(ident.Pos()); there.Filename == at.Filename && there.Line == at.Line {
				return declared
			}
		}
	}
	return of
}

// qualified returns the name of an object qualified by the declarations that contain it, as
// the parser qualifies it:
//
//   - a method by the type of its receiver
//   - a field, or a method of an interface, by the type that declares it
//   - a parameter or a local by its function
//
// An object of a file of which the view has no syntax, such as a field of the standard
// library, keeps its name. A method keeps the qualifier of its receiver there too.
func (*Engine) qualified(v *view, of types.Object) string {
	if fn, is := of.(*types.Func); is {
		if recv := fn.Signature().Recv(); recv != nil {
			if named := typeName(recv.Type()); named != "" {
				return sema.Qualify(named, of.Name())
			}
		}
	}
	file := v.file(of.Pos())
	if file == nil {
		return of.Name()
	}
	container := ""
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil || n.Pos() > of.Pos() || n.End() < of.Pos() {
			return false
		}
		switch held := n.(type) {
		case *ast.FuncDecl:
			if held.Name.Pos() != of.Pos() {
				container = funcName(held)
			}
		case *ast.TypeSpec:
			if held.Name.Pos() != of.Pos() {
				container = sema.Qualify(container, held.Name.Name)
			}
		}
		return true
	})
	return sema.Qualify(container, of.Name())
}

// typeName returns the name of the named type that t is or points to, or the empty string for
// another type.
func typeName(t types.Type) string {
	if pointer, is := t.(*types.Pointer); is {
		t = pointer.Elem()
	}
	if named, is := t.(*types.Named); is {
		return named.Obj().Name()
	}
	return ""
}

// funcName returns the qualified name of a function declaration: the name of a method
// qualified by the type of its receiver, and the name of a function.
func funcName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	return sema.Qualify(receiverName(decl.Recv.List[0].Type), decl.Name.Name)
}

// receiverName returns the name of the type of a receiver, through a pointer and through type
// arguments.
func receiverName(expr ast.Expr) string {
	switch held := expr.(type) {
	case *ast.StarExpr:
		return receiverName(held.X)
	case *ast.IndexExpr:
		return receiverName(held.X)
	case *ast.IndexListExpr:
		return receiverName(held.X)
	case *ast.Ident:
		return held.Name
	}
	return ""
}

// kindOf returns the kind of an object of the type checker, and reports whether the object has
// one. A function with a receiver is a method, and a type name has the kind of the type it
// names, by [underlying]. A builtin and the untyped nil have no kind.
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

// underlying returns the kind of a type declaration by the type it names: a struct, an
// interface, or another type.
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

// signature returns the declaration of an object without its body, as the type checker writes
// it, relative to the package that declares it. It returns the first line of a declaration
// that spans lines, such as a struct type.
func signature(of types.Object) string {
	held := types.ObjectString(of, types.RelativeTo(of.Pkg()))
	if at := strings.IndexByte(held, '\n'); at >= 0 {
		held = held[:at]
	}
	return held
}

// sources are the contents of the files that one answer cites, by absolute path.
type sources map[string][]byte

// read returns the content of the file at full. It reads the file from disk the first time,
// and returns no content for a file that cannot be read.
func (s sources) read(full string) []byte {
	content, read := s[full]
	if !read {
		content, _ = os.ReadFile(full)
		s[full] = content
	}
	return content
}

// sited returns the span of width bytes at a position, and the source line that the position
// is on, without the white space around it.
func (e *Engine) sited(v *view, files sources, at token.Pos, width int) (source.Span, string) {
	if !at.IsValid() {
		return source.Span{}, ""
	}
	held := v.fset.Position(at)
	span := source.Span{
		Path:  e.pathOf(held.Filename),
		Start: source.Position{Offset: held.Offset, Line: held.Line - 1, Column: held.Column - 1},
		End: source.Position{
			Offset: held.Offset + width,
			Line:   held.Line - 1,
			Column: held.Column - 1 + width,
		},
	}
	return span, strings.TrimSpace(lang.LineAt(files.read(held.Filename), held.Offset))
}

// enclosing returns the innermost declaration that contains a position of pkg, and reports
// whether one does. A use in a field of a struct belongs to the field. A use in the signature
// or the body of a function belongs to the function, because a local is not a declaration that
// an answer names.
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

// declares returns the name that a node declares at the position at, or nil for a node that
// declares nothing there. A field list of a struct or an interface declares its members. The
// parameters and the results of a function declare nothing, because [Engine.enclosing] stops at
// the function.
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

// member returns the name of the field or the method of fields that contains at, or nil for
// none.
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

// first returns the first of names, or nil for an embedded field, which does not write a name.
func first(names []*ast.Ident) *ast.Ident {
	if len(names) == 0 {
		return nil
	}
	return names[0]
}
