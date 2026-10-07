// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"go/scanner"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// The notes of the caveats of the files that the build constraints exclude, as formats of
// [naming]. The note of one file names it. The note of two or more files names their number and
// the first of them, because the text of an answer states the note of a caveat without its
// paths.
const (
	excludedOne  = "the build constraints exclude %s from the build that the answer reads"
	excludedMany = "the build constraints exclude %d files from the build that the answer reads, such as %s"
	uncheckedOne = "the build constraints exclude %s, which can use a changed package, from the build that " +
		"the check reads"
	uncheckedMany = "the build constraints exclude %d files that can use a changed package from the build that " +
		"the check reads, such as %s"
	unbuiltOne   = "the build constraints can exclude %s from every build that the server checks"
	unbuiltMany  = "the build constraints can exclude %d files from every build that the server checks, such as %s"
	unrenamedOne = "the plan does not rename the declaration in %s, which the build constraints exclude from " +
		"the build that the plan reads"
	unrenamedMany = "the plan does not rename the declaration in %d files that the build constraints exclude from " +
		"the build that the plan reads, such as %s"
)

// readElsewhere and renamedElsewhere are the notes of the caveats of a declaration in a file of
// another port, as formats of the path of the file.
const (
	readElsewhere = "the server reads %s in the build of another port, and the build constraints can exclude " +
		"other files from that build"
	renamedElsewhere = "the server renames the declaration in %s in the build of another port, and the build " +
		"constraints can exclude other files that name it from that build"
)

// port is a target of the go command: an operating system and an architecture, such as linux
// and amd64.
type port struct {
	goos, goarch string
}

// ports are the ports of the go command of Go 1.27, in the order of go tool dist list. gopls
// tries every one of them for a file that the default port excludes. A test compares the list
// with go tool dist list of the toolchain that runs the tests.
var ports = []port{
	{"aix", "ppc64"},
	{"android", "386"},
	{"android", "amd64"},
	{"android", "arm"},
	{"android", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"dragonfly", "amd64"},
	{"freebsd", "386"},
	{"freebsd", "amd64"},
	{"freebsd", "arm"},
	{"freebsd", "arm64"},
	{"illumos", "amd64"},
	{"ios", "amd64"},
	{"ios", "arm64"},
	{"js", "wasm"},
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"linux", "loong64"},
	{"linux", "mips"},
	{"linux", "mips64"},
	{"linux", "mips64le"},
	{"linux", "mipsle"},
	{"linux", "ppc64"},
	{"linux", "ppc64le"},
	{"linux", "riscv64"},
	{"linux", "s390x"},
	{"netbsd", "386"},
	{"netbsd", "amd64"},
	{"netbsd", "arm"},
	{"netbsd", "arm64"},
	{"openbsd", "386"},
	{"openbsd", "amd64"},
	{"openbsd", "arm"},
	{"openbsd", "arm64"},
	{"openbsd", "ppc64"},
	{"openbsd", "riscv64"},
	{"plan9", "386"},
	{"plan9", "amd64"},
	{"plan9", "arm"},
	{"solaris", "amd64"},
	{"wasip1", "wasm"},
	{"windows", "386"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

// Unread returns the caveat of an answer of a language server of Go, such as gopls, to the
// relations of kind of the declaration that of identifies, by the rule of [Engine.unread]. The
// declaration is in the file of req.Declared, or in the directory that the unit of the ID names
// when req does not declare a span. A test file counts when req includes tests.
//
// The caveat is a [trust.CaveatInactiveBuild] caveat that names the files of the rule, or one
// that states that the build constraints can exclude other files from the build of another
// port, when the server reads the declaration in that build. Unread returns no caveat when no
// excluded file can contain such a relation.
//
// It returns the error of the walk of the workspace and of go list.
func (e *Engine) Unread(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) ([]trust.Caveat, error) {
	left, elsewhere, err := e.unread(ctx, req.Declared.Path, of, kind, req.Tests)
	switch {
	case err != nil:
		return nil, err
	case elsewhere:
		note := fmt.Sprintf(readElsewhere, req.Declared.Path)
		return []trust.Caveat{{Code: trust.CaveatInactiveBuild, Note: note}}, nil
	}
	return inactive(left), nil
}

// Unrenamed returns the caveat of a plan of a language server of Go, such as gopls, that renames
// the declaration of target, by the rule of [Engine.unread] for the uses of the declaration and
// with the test files. The server renames the declaration in the files of the build that it
// reads, so the plan leaves a use in an excluded file as it is.
//
// The caveat is a [trust.CaveatUnrewritten] caveat that names the files of the rule, or one that
// states that the build constraints can exclude other files that name the declaration from the
// build of another port, when the server renames the declaration in that build. Unrenamed
// returns no caveat when no excluded file names the declaration.
//
// It returns the error of the walk of the workspace and of go list.
func (e *Engine) Unrenamed(ctx context.Context, target edit.Target) ([]trust.Caveat, error) {
	left, elsewhere, err := e.unread(ctx, target.Span.Path, target.Symbol, sema.ReferencedBy, true)
	switch {
	case err != nil:
		return nil, err
	case elsewhere:
		note := fmt.Sprintf(renamedElsewhere, target.Span.Path)
		return []trust.Caveat{{Code: trust.CaveatUnrewritten, Note: note}}, nil
	}
	return naming(trust.CaveatUnrewritten, left, unrenamedOne, unrenamedMany), nil
}

// Unverified returns the caveat of an answer of a language server of Go, such as gopls, to a
// verify of the scope of req: a [trust.CaveatInactiveBuild] caveat that names the Go files in
// the scope that the build constraints of the load exclude and that no build of another port
// includes for certain, by the rule of [built], or none. The server checks a file of another
// port in the build of that port. A test file counts when req includes tests.
//
// It returns the error of the walk of the workspace and of go list.
func (e *Engine) Unverified(ctx context.Context, req engine.Request) ([]trust.Caveat, error) {
	g, err := e.listedNow(ctx)
	if err != nil {
		return nil, err
	}
	var left []source.Path
	for _, p := range e.within(e.kept(g.excluded(), req.Tests), scoped(req.Scope)) {
		if !built(e.fullPath(p)) {
			left = append(left, p)
		}
	}
	return naming(trust.CaveatInactiveBuild, left, unbuiltOne, unbuiltMany), nil
}

// Unchecked returns the caveat of a check of the change to files by any engine of Go: a
// [trust.CaveatDependents] caveat that names the Go files that the build constraints of the load
// exclude and that can use the package of a changed file, by the rule of [Engine.dependents],
// or none.
//
// It returns the error of the walk of the workspace and of go list.
func (e *Engine) Unchecked(ctx context.Context, files map[source.Path][]byte) ([]trust.Caveat, error) {
	g, err := e.listedNow(ctx)
	if err != nil {
		return nil, err
	}
	return unchecked(e.dependents(g, slices.Collect(maps.Keys(files)))), nil
}

// unread returns the workspace paths of the Go files that the build constraints of the load
// exclude and that can contain a relation of kind of the declaration that of identifies, by the
// rule of [Engine.leftOut]. The declaration is in the file at declared, or in the directory
// that the unit of the ID names when declared is empty. A test file counts when tests is true.
//
// A language server reads a declaration in a file of the default build in that build, as the
// type checker does. It reads a declaration in a file that the default build excludes in the
// build of another port when one includes the file, by the rule of [built], and unread then
// reports true. The server chooses that port, so the files that its build excludes are unknown.
// No file is at an empty path, so a declaration without a file is in the default build.
//
// It returns the error of the walk of the workspace and of go list.
func (e *Engine) unread(
	ctx context.Context,
	declared source.Path,
	of sema.ID,
	kind sema.RelationKind,
	tests bool,
) ([]source.Path, bool, error) {
	g, err := e.listedNow(ctx)
	if err != nil {
		return nil, false, err
	}
	dir, full := e.fullPath(of.Unit()), ""
	if declared != "" {
		full = e.fullPath(declared)
		dir = filepath.Dir(full)
	}
	left := e.leftOut(g, dir, of.Base(), kind, tests)
	return left, len(left) > 0 && built(full), nil
}

// viewed returns the view of the whole workspace that w describes, by the rule of
// [Engine.current], and its graph, by the rule of [Engine.listedOf]. It returns the error of
// either.
func (e *Engine) viewed(ctx context.Context, w walked) (*view, *graph, error) {
	v, err := e.current(ctx, w)
	if err != nil {
		return nil, nil, err
	}
	g, err := e.listedOf(ctx, w)
	return v, g, err
}

// listedNow returns the graph of the workspace as it is on disk, by the rule of
// [Engine.listedOf].
func (e *Engine) listedNow(ctx context.Context) (*graph, error) {
	w, err := e.walk()
	if err != nil {
		return nil, err
	}
	return e.listedOf(ctx, w)
}

// listedOf returns the graph of the workspace that w describes, by the rule of [Engine.graphOf].
func (e *Engine) listedOf(ctx context.Context, w walked) (*graph, error) {
	plans, _, err := e.planning(ctx, w.modules)
	if err != nil {
		return nil, err
	}
	return e.graphOf(ctx, w, plans)
}

// leftOut returns the workspace paths of the files of g that the build constraints of the load
// exclude and that can contain a relation of kind of the declaration name in dir, an absolute
// path:
//
//   - none for [sema.Calls] and [sema.Embeds], whose sites are in the declaration
//   - every excluded file for [sema.Implements] and [sema.ImplementedBy], because a type
//     satisfies an interface by its methods alone
//   - for the other kinds, the files that [graph.reaching] returns for the packages in dir and
//     that contain name as an identifier, because each such relation is at a use of the name
//
// A test file counts when tests is true.
func (e *Engine) leftOut(g *graph, dir, name string, kind sema.RelationKind, tests bool) []source.Path {
	excluded := e.kept(g.excluded(), tests)
	switch kind {
	case sema.Calls, sema.Embeds:
		return nil
	case sema.Implements, sema.ImplementedBy:
		return e.workspacePaths(excluded)
	}
	reached := g.reaching(excluded, g.housing([]string{dir}), []string{dir})
	return e.workspacePaths(slices.DeleteFunc(reached, func(full string) bool { return !mentions(full, name) }))
}

// dependents returns the workspace paths of the files of g that the build constraints of the
// load exclude and that can use the package of a file of changed, by the rule of
// [graph.reaching]. A file of changed does not count, as a file that a change deletes does
// not.
func (e *Engine) dependents(g *graph, changed []source.Path) []source.Path {
	full := make([]string, 0, len(changed))
	for _, p := range changed {
		full = append(full, e.fullPath(p))
	}
	dirs := dirsOf(full)
	reached := g.reaching(g.excluded(), g.housing(dirs), dirs)
	return e.workspacePaths(slices.DeleteFunc(reached, func(one string) bool { return slices.Contains(full, one) }))
}

// kept returns the files of full, absolute paths, without the test files unless tests is true.
// The files of a graph are in the workspace, because go list lists the packages under the root
// alone.
func (e *Engine) kept(full []string, tests bool) []string {
	var out []string
	for _, one := range full {
		if tests || !e.declared.IsTest(string(e.pathOf(one))) {
			out = append(out, one)
		}
	}
	return out
}

// within returns the workspace paths of the files of full, absolute paths, that are in scope.
func (e *Engine) within(full []string, scope source.Path) []source.Path {
	var out []source.Path
	for _, p := range e.workspacePaths(full) {
		if lang.Within(p, scope) {
			out = append(out, p)
		}
	}
	return out
}

// workspacePaths returns the workspace path of each file of full, absolute paths, in order.
func (e *Engine) workspacePaths(full []string) []source.Path {
	out := make([]source.Path, 0, len(full))
	for _, one := range full {
		out = append(out, e.pathOf(one))
	}
	return out
}

// mentions reports whether the Go file at the absolute path full contains name as a token of
// go/scanner, which leaves out comments. A token with the text of a name is an identifier,
// because no keyword, literal or operator of Go has the text of a declared name. A file that
// cannot be read mentions every name, because its content is unknown.
func mentions(full, name string) bool {
	content, err := os.ReadFile(full)
	if err != nil {
		return true
	}
	var tokens scanner.Scanner
	tokens.Init(token.NewFileSet().AddFile(full, -1, len(content)), content, nil, 0)
	for {
		_, tok, literal := tokens.Scan()
		switch {
		case tok == token.EOF:
			return false
		case literal == name:
			return true
		}
	}
}

// built reports whether a language server of Go builds the Go file at the absolute path full in
// the build of another port of [ports]. gopls reads a file that the default port excludes in
// the build of a port whose build constraints match the file, and the go command builds another
// port with cgo off unless the environment turns cgo on. built reports true only when every
// build that the server can choose includes the file:
//
//   - the build constraints of go/build exclude the file from the default port, whose build the
//     server would read it in
//   - the build constraints match the file for a port of ports
//   - each port includes the file with cgo on exactly when it includes it with cgo off
//   - the file does not import C, because the go command compiles such a file only with cgo on
//
// go/build evaluates the build constraints without the tags of GOFLAGS. A file that cannot be
// read, or whose header go/build cannot read, is not built.
func built(full string) bool {
	content, err := os.ReadFile(full)
	if err != nil || slices.Contains(importsOf(full), "C") {
		return false
	}
	matches := func(at build.Context) bool {
		//dokimi:mutate-skip sbr-delete: MatchFile reads the same content from the file without it, once a port
		at.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(content)), nil }
		match, err := at.MatchFile(filepath.Dir(full), filepath.Base(full))
		return err == nil && match
	}
	if matches(build.Default) {
		return false
	}
	some := false
	for _, one := range ports {
		on, off := build.Default, build.Default
		on.GOOS, on.GOARCH, on.CgoEnabled = one.goos, one.goarch, true
		off.GOOS, off.GOARCH, off.CgoEnabled = one.goos, one.goarch, false
		switch in := matches(on); {
		case in != matches(off):
			return false
		case in:
			some = true
		}
	}
	return some
}

// inactive returns the caveat of an answer that does not read the files at left, which the
// build constraints exclude from the build that the answer reads, or none for no file.
func inactive(left []source.Path) []trust.Caveat {
	return naming(trust.CaveatInactiveBuild, left, excludedOne, excludedMany)
}

// unchecked returns the caveat of a check that does not check the files at left, which can use
// a changed package and which the build constraints exclude from the build that the check
// reads, or none for no file.
func unchecked(left []source.Path) []trust.Caveat {
	return naming(trust.CaveatDependents, left, uncheckedOne, uncheckedMany)
}

// naming returns a caveat of code that names the files at left, or none for no file. The note
// is the format one with the path of the one file, or the format many with the number of files
// and the path of the first.
func naming(code trust.CaveatCode, left []source.Path, one, many string) []trust.Caveat {
	switch len(left) {
	case 0:
		return nil
	case 1:
		return []trust.Caveat{{Code: code, Note: fmt.Sprintf(one, left[0]), Paths: left}}
	}
	return []trust.Caveat{{Code: code, Note: fmt.Sprintf(many, len(left), left[0]), Paths: left}}
}
