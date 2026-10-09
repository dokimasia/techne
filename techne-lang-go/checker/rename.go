// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"cmp"
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"slices"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// unrenamedOne and unrenamedMany are the notes of the caveat of a rename that leaves the
// declaration in files that the build constraints exclude. They are formats of [naming].
// renamedElsewhere is the note of the caveat of a declaration in a file of another port. Its
// format takes the path of the file.
const (
	unrenamedOne = "the plan does not rename the declaration in %s, which the build constraints exclude from " +
		"the build that the plan reads"
	unrenamedMany = "the plan does not rename the declaration in %d files that the build constraints exclude from " +
		"the build that the plan reads, such as %s"
	renamedElsewhere = "the server renames the declaration in %s in the build of another port, and the build " +
		"constraints can exclude other files that name it from that build"
)

// Renamed returns the changes that rename to fresh the uses of the declaration of target in the
// Go files that the build constraints exclude from the default build. It also returns the caveat
// of the excluded files that it leaves as they are. A language server of Go, such as gopls,
// renames the declaration in the files of the default build alone.
//
// Renamed reads the excluded files that [Engine.unread] returns for the uses of the declaration.
// A test file counts. Renamed type-checks each file in the build of the port that [portOf]
// returns for it. One load reads the files of a port together with the package of the
// declaration. [Engine.object] finds the declaration in the load by the ID of target in the file
// of its span. Renamed rewrites each name in the file that the type checker binds to the
// declaration. It also rewrites the name of each field that embeds the declaration.
//
// The caveat lists the files that keep their uses. A file keeps its uses in these cases:
//
//   - Every build with cgo off excludes the file. Such a file imports C or needs a build tag that
//     the build leaves unset.
//   - The build of its port does not load.
//   - The build of its port reports a type error in the file.
//   - The build of its port excludes the file of the declaration.
//   - The declaration is a method and the file declares a method of the same name. A rename of
//     the method can rename such a method with it.
//
// The caveat is a [trust.CaveatUnrewritten] caveat. The server renames a declaration in a file of
// another port in the build of that port. For such a declaration Renamed returns no change and a
// caveat without paths. It returns no change and no caveat when no excluded file contains the
// name of the declaration.
//
// It returns the error of the walk of the workspace and of go list.
func (e *Engine) Renamed(ctx context.Context, target edit.Target, fresh string) ([]edit.Change, []trust.Caveat, error) {
	w, err := e.walk()
	if err != nil {
		return nil, nil, err
	}
	plans, goroot, g, err := e.listedWith(ctx, w)
	if err != nil {
		return nil, nil, err
	}
	left, elsewhere := e.unreadIn(g, target.Span.Path, target.Symbol, sema.ReferencedBy, true)
	if elsewhere {
		note := fmt.Sprintf(renamedElsewhere, target.Span.Path)
		return nil, []trust.Caveat{{Code: trust.CaveatUnrewritten, Note: note}}, nil
	}

	ported := map[port][]string{}
	var order []port
	var rest []source.Path
	for _, p := range left {
		full := e.fullPath(p)
		at, built := portOf(full)
		switch {
		case !built:
			rest = append(rest, p)
			continue
		case ported[at] == nil:
			order = append(order, at)
		}
		ported[at] = append(ported[at], full)
	}
	_, dir := e.declaredIn(target.Span.Path, target.Symbol)
	var changes []edit.Change
	for _, at := range order {
		renamed, kept := e.renamedIn(ctx, plans, goroot, g, at, dir, ported[at], target, fresh)
		changes = append(changes, renamed...)
		rest = append(rest, kept...)
	}
	slices.SortFunc(changes, func(a, b edit.Change) int { return cmp.Compare(a.Path, b.Path) })
	slices.Sort(rest)
	return changes, naming(trust.CaveatUnrewritten, rest, unrenamedOne, unrenamedMany), nil
}

// renamedIn renames the uses of the declaration of target in files by the rules of
// [Engine.Renamed]. files are the absolute paths of excluded files that the build of the port at
// includes. dir is the directory of the declaration. renamedIn returns the changes that rename
// the uses to fresh and the workspace paths of the files that it leaves as they are. One load of
// the build of the port reads the packages of dir and of files, by the rule of [graph.porting].
func (e *Engine) renamedIn(
	ctx context.Context,
	plans []plan,
	goroot string,
	g *graph,
	at port,
	dir string,
	files []string,
	target edit.Target,
	fresh string,
) ([]edit.Change, []source.Path) {
	v, err := e.loaded(ctx, g.porting(plans, append(dirsOf(files), dir), at), goroot, nil)
	var subject types.Object
	//dokimi:mutate-skip ror-true: the load of a port fails only when the go command fails after the listing
	if err == nil {
		// object returns no declaration for none and for two or more, and the rename leaves the
		// files as they are in both cases.
		subject, _, _ = e.object(v, engine.Request{Scope: target.Span.Path, Tests: true}, target.Symbol)
	}
	if subject == nil {
		return nil, e.workspacePaths(files)
	}

	// fit reports for each file that the load type-checked whether the rename rewrites it: the
	// file has no type error, and [clashes] reports false for it. A file that the load did not
	// read is not in fit. The load reads a file of a package twice when the package has a test
	// variant.
	fit := map[string]bool{}
	edits := map[source.Path]map[int]edit.TextEdit{}
	cited := sources{}
	for _, pkg := range v.held() {
		for i, f := range pkg.Syntax {
			fit[v.sourceOf(pkg, i)] = !clashes(f, subject)
		}
		for name, used := range pkg.TypesInfo.Uses {
			if !v.binds(used, subject) {
				continue
			}
			span, _ := e.sited(v, cited, name.Pos(), len(name.Name))
			if edits[span.Path] == nil {
				edits[span.Path] = map[int]edit.TextEdit{}
			}
			edits[span.Path][span.Start.Offset] = edit.TextEdit{Span: span, New: fresh}
		}
	}
	for _, pkg := range v.held() {
		for _, terr := range pkg.TypeErrors {
			fit[v.fset.Position(terr.Pos).Filename] = false
		}
	}

	var out []edit.Change
	var left []source.Path
	for _, full := range files {
		p := e.pathOf(full)
		switch {
		case !fit[full]:
			left = append(left, p)
		case len(edits[p]) > 0:
			one := edit.Change{Kind: edit.ChangeEdit, Path: p}
			for _, offset := range slices.Sorted(maps.Keys(edits[p])) {
				one.Edits = append(one.Edits, edits[p][offset])
			}
			out = append(out, one)
		}
	}
	return out, left
}

// binds reports whether a rename of subject rewrites the name of a use of used. It does when
// [view.same] reports that used is subject. It also does when used is a field that embeds
// subject, a defined type or an alias, or a pointer to one, because such a field has the name
// of the type.
func (v *view) binds(used, subject types.Object) bool {
	if v.same(used, subject) {
		return true
	}
	field, isField := used.(*types.Var)
	if !isField || !field.Embedded() {
		return false
	}
	embedded := field.Type()
	if pointer, isPointer := embedded.(*types.Pointer); isPointer {
		embedded = pointer.Elem()
	}
	if named, isNamed := embedded.(interface{ Obj() *types.TypeName }); isNamed {
		return v.same(named.Obj(), subject)
	}
	return false
}

// clashes reports whether the file f declares a method of a type or of an interface with the
// name of subject. It reports false when subject is not a method. A rename of the method can
// rename such a method with it. Renamed leaves such a file as it is.
func clashes(f *ast.File, subject types.Object) bool {
	method, isFunc := subject.(*types.Func)
	if !isFunc || method.Signature().Recv() == nil {
		return false
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch held := n.(type) {
		case *ast.FuncDecl:
			found = found || held.Recv != nil && held.Name.Name == method.Name()
		case *ast.InterfaceType:
			for _, field := range held.Methods.List {
				found = found || slices.ContainsFunc(field.Names, func(name *ast.Ident) bool {
					return name.Name == method.Name()
				})
			}
		}
		return !found
	})
	return found
}
