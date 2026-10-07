// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// listing is the template of go list that writes one package per line, with tabs between the
// fields: the import path, the directory, the packages that the package and its tests import,
// and then each Go file of the directory that the build constraints exclude, such as a file of
// another operating system.
const listing = `{{.ImportPath}}	{{.Dir}}	{{join .Imports " "}} {{join .TestImports " "}} ` +
	`{{join .XTestImports " "}}	{{join .IgnoredGoFiles "\t"}}`

// entry is the line of one package in the output of go list with the template listing: the
// import path, the directory, the import paths that the package and its tests import, and the
// absolute paths of the Go files of the package that the build constraints exclude. The pattern
// of a directory outside the main modules of the go command gives a line without a directory
// and without files.
type entry struct {
	path, dir string
	imports   []string
	ignored   []string
}

// entries returns the packages of out, the output of go list with the template listing, in the
// order of its lines.
func entries(out string) []entry {
	var found []entry
	for line := range strings.Lines(out) {
		path, rest, _ := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		dir, rest, _ := strings.Cut(rest, "\t")
		imported, ignored, _ := strings.Cut(rest, "\t")
		pkg := entry{path: path, dir: dir, imports: strings.Fields(imported)}
		for name := range strings.SplitSeq(ignored, "\t") {
			if name != "" {
				pkg.ignored = append(pkg.ignored, filepath.Join(dir, name))
			}
		}
		found = append(found, pkg)
	}
	return found
}

// graph is the packages of the workspace and what each imports, from the go list of each plan.
// It is the part of a load that names packages, without their syntax and their types.
type graph struct {
	// stamp is the stamp of the walk that the graph describes.
	stamp string
	// dirs are the directories of the packages, and imports what each package and its tests
	// import, by import path.
	dirs    map[string]string
	imports map[string][]string
	// ignored are the absolute paths of the Go files that the build constraints exclude from
	// each package, by import path, the bare packages included.
	ignored map[string][]string
	// listed are the import paths that each plan lists, by the index of the plan. unlisted
	// reports that go list failed for the plan, whose load then reports the failure.
	listed   [][]string
	unlisted []bool
	// bare are the packages of each plan whose Go files the build constraints all exclude from
	// the default build, such as a package of windows alone, by the index of the plan. A pattern
	// that ends in /... leaves out the directory of such a package, so the graph lists the
	// directory by its path. ignored contains the files of a bare package, and the other fields
	// leave it out.
	bare [][]entry
}

// graphOf returns the graph of the workspace that w describes, from the cached graph when its
// stamp is the stamp of w. It lists the packages of each plan, with the go command in the
// directory and the environment of the plan. A plan whose listing fails, such as a module whose
// go.mod file does not parse, is unlisted, and the graph contains none of its packages. It then
// lists the directories that [Engine.missing] returns, which are the directories of the bare
// packages.
func (e *Engine) graphOf(ctx context.Context, w walked, plans []plan) (*graph, error) {
	e.listing.Lock()
	defer e.listing.Unlock()
	if e.graph != nil && e.graph.stamp == w.stamp {
		return e.graph, nil
	}
	g := &graph{
		stamp:   w.stamp,
		dirs:    map[string]string{},
		imports: map[string][]string{},
		ignored: map[string][]string{},
		bare:    make([][]entry, len(plans)),
	}
	for _, one := range plans {
		args := append([]string{"list", "-e", "-f", listing}, one.patterns...)
		out, err := goIn(ctx, one.dir, environ(one.alone), args...)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		g.unlisted = append(g.unlisted, err != nil)
		var listed []string
		for _, pkg := range entries(out) {
			g.dirs[pkg.path], g.imports[pkg.path], g.ignored[pkg.path] = pkg.dir, pkg.imports, pkg.ignored
			listed = append(listed, pkg.path)
		}
		g.listed = append(g.listed, listed)
	}
	for i, dirs := range e.missing(w, g, plans) {
		one := plans[i]
		// goIn returns the empty output for a listing that fails, such as the listing of a plan
		// whose go.mod file does not parse.
		out, _ := goIn(ctx, one.dir, environ(one.alone), append([]string{"list", "-e", "-f", listing}, dirs...)...)
		g.bare[i] = entries(out)
		for _, pkg := range g.bare[i] {
			g.ignored[pkg.path] = pkg.ignored
		}
	}
	//dokimi:mutate-skip ror-false,sbr-delete: only a context that ends during a listing reaches this, which no test can time
	if ctx.Err() != nil {
		//dokimi:mutate-skip sbr-zero: only a context that ends during a listing reaches this, which no test can time
		return nil, ctx.Err()
	}
	e.graph = g
	return g, nil
}

// missing returns the directories of the Go files of w that no package of g is in, by the index
// of the plans whose directories contain them. A directory is a pattern of go list relative to
// the directory of the plan. go list leaves such a directory out of a pattern that ends in /...,
// because the build constraints exclude every Go file in it. A pattern of the directory alone
// lists it. The listing of a plan whose main modules do not contain the directory gives a line
// without a directory.
func (e *Engine) missing(w walked, g *graph, plans []plan) map[int][]string {
	listed := slices.Sorted(maps.Values(g.dirs))
	dirs := map[string]bool{}
	for _, p := range w.files {
		dir := filepath.Dir(e.fullPath(p))
		if _, found := slices.BinarySearch(listed, dir); !found {
			dirs[dir] = true
		}
	}
	out := map[int][]string{}
	for _, dir := range slices.Sorted(maps.Keys(dirs)) {
		for i, one := range plans {
			if below(dir, one.dir) {
				relative, _ := filepath.Rel(one.dir, dir)
				out[i] = append(out[i], "./"+filepath.ToSlash(relative))
			}
		}
	}
	return out
}

// affected returns the import paths of the packages that a change of the files at full, absolute
// paths, can break: each package in the directory of a file, and each package that imports one
// of them, directly or through another package of the workspace. A test that imports a package
// is part of the package that it tests.
func (g *graph) affected(full []string) []string {
	return slices.Sorted(maps.Keys(g.importing(g.housing(dirsOf(full)))))
}

// housing returns the import paths of the packages in dirs, absolute paths.
func (g *graph) housing(dirs []string) []string {
	var out []string
	for path, dir := range g.dirs {
		if slices.Contains(dirs, dir) {
			out = append(out, path)
		}
	}
	return out
}

// dirsOf returns the directory of each file of full, absolute paths, in order.
func dirsOf(full []string) []string {
	out := make([]string, 0, len(full))
	for _, one := range full {
		out = append(out, filepath.Dir(one))
	}
	return out
}

// importing returns the import paths of seeds and of each package of the workspace that imports
// one of them, directly or through another package of the workspace. A test that imports a
// package is part of the package that it tests.
func (g *graph) importing(seeds []string) map[string]bool {
	importers := map[string][]string{}
	for path, imported := range g.imports {
		for _, one := range imported {
			importers[one] = append(importers[one], path)
		}
	}
	reached := map[string]bool{}
	queue := slices.Clone(seeds)
	for _, path := range seeds {
		reached[path] = true
	}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		for _, importer := range importers[path] {
			if !reached[importer] {
				reached[importer] = true
				queue = append(queue, importer)
			}
		}
	}
	return reached
}

// excluded returns the absolute paths of the Go files that the build constraints exclude from
// the packages of the workspace, such as a file of another operating system, each once, sorted.
// The type checker reads none of them.
func (g *graph) excluded() []string {
	var out []string
	for _, files := range g.ignored {
		out = append(out, files...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// reaching returns the files of excluded, absolute paths, that can use a declaration of the
// packages at seeds. The dependent packages are the packages of seeds and each package that
// imports one of them, by the rule of [graph.importing]. A file can use the declaration when it
// is in the directory of a dependent package or imports one:
//
//   - a file names a declaration of another package only through an import of that package
//   - a file uses a method or a field of a type of another package through a value, which an
//     expression of its own package or of a package that it imports can have
//
// dirs are the directories of the packages of seeds, which a package that go list does not
// list, such as an external test package, has too.
func (g *graph) reaching(excluded, seeds, dirs []string) []string {
	reached := g.importing(seeds)
	near := slices.Clone(dirs)
	for path := range reached {
		near = append(near, g.dirs[path])
	}
	var out []string
	for _, full := range excluded {
		if slices.Contains(near, filepath.Dir(full)) ||
			slices.ContainsFunc(importsOf(full), func(path string) bool { return reached[path] }) {
			out = append(out, full)
		}
	}
	return out
}

// under returns the import paths of the packages of scope, a workspace path under root: the
// packages in the directory of a file, or the packages of a directory and of its
// subdirectories.
func (g *graph) under(root string, scope source.Path, file bool) []string {
	full := filepath.Join(root, filepath.FromSlash(string(scope)))
	var out []string
	for path, dir := range g.dirs {
		if (file && dir == filepath.Dir(full)) || (!file && below(dir, full)) {
			out = append(out, path)
		}
	}
	slices.Sort(out)
	return out
}

// narrowed returns the plans with each of their patterns replaced by the import paths of
// wanted that the plan lists, and without a plan that lists none of them. A plan whose packages
// are all wanted keeps its patterns, and so does an unlisted plan, whose load reports why it
// fails. It reports whether each plan kept its patterns.
func (g *graph) narrowed(plans []plan, wanted []string) ([]plan, bool) {
	keep := map[string]bool{}
	for _, one := range wanted {
		keep[one] = true
	}
	var out []plan
	whole := true
	for i, one := range plans {
		var patterns []string
		for _, path := range g.listed[i] {
			if keep[path] {
				patterns = append(patterns, path)
			}
		}
		switch {
		case g.unlisted[i]:
		case len(patterns) == 0:
			whole = false
			continue
		case len(patterns) < len(g.listed[i]):
			one.patterns, whole = patterns, false
		}
		out = append(out, one)
	}
	return out, whole
}

// viewing returns a view of the packages that wanted selects from the graph of w, with overlay
// in place of the files that it names. The cached view answers a question without an overlay
// when its stamp is the stamp of w and it contains every package that wanted selects. A
// question whose packages the graph does not list, such as the files of a new directory, reads
// a view of the whole workspace. A view without an overlay replaces the cached view.
func (e *Engine) viewing(
	ctx context.Context,
	w walked,
	overlay map[string][]byte,
	wanted func(*graph) []string,
) (*view, error) {
	plans, goroot, err := e.planning(ctx, w.modules)
	if err != nil {
		return nil, err
	}
	g, err := e.graphOf(ctx, w, plans)
	if err != nil {
		return nil, err
	}
	selected := wanted(g)
	if overlay == nil {
		if cached := e.cached(w); cached != nil && cached.contains(selected) {
			return cached, nil
		}
	}
	whole := true
	if narrowed, all := g.narrowed(plans, selected); len(narrowed) > 0 {
		plans, whole = narrowed, all
	}
	v, err := e.loaded(ctx, plans, goroot, overlay)
	if err != nil || overlay != nil {
		return v, err
	}
	v.stamp, v.whole = w.stamp, whole
	e.keep(v)
	return v, nil
}
