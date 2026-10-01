// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// listing is the template of go list that writes one package per line: the import path, the
// directory, and the packages that the package and its tests import, separated by tabs.
const listing = `{{.ImportPath}}	{{.Dir}}	{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`

// graph is the packages of the workspace and what each imports, from one go list of each plan.
// It is the part of a load that names packages, without their syntax and their types.
type graph struct {
	// stamp is the stamp of the walk that the graph describes.
	stamp string
	// dirs are the directories of the packages, and imports what each package and its tests
	// import, by import path.
	dirs    map[string]string
	imports map[string][]string
	// listed are the import paths that each plan lists, by the index of the plan. unlisted
	// reports that go list failed for the plan, whose load then reports the failure.
	listed   [][]string
	unlisted []bool
}

// graphOf returns the graph of the workspace that w describes, from the cached graph when its
// stamp is the stamp of w. It lists the packages of each plan, with the go command in the
// directory and the environment of the plan. A plan whose listing fails, such as a module whose
// go.mod file does not parse, lists no package and is unlisted.
func (e *Engine) graphOf(ctx context.Context, w walked, plans []plan) (*graph, error) {
	e.listing.Lock()
	defer e.listing.Unlock()
	if e.graph != nil && e.graph.stamp == w.stamp {
		return e.graph, nil
	}
	g := &graph{stamp: w.stamp, dirs: map[string]string{}, imports: map[string][]string{}}
	for _, one := range plans {
		args := append([]string{"list", "-e", "-f", listing}, one.patterns...)
		out, err := goIn(ctx, one.dir, environ(one.alone), args...)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		g.unlisted = append(g.unlisted, err != nil)
		var listed []string
		lines := bufio.NewScanner(strings.NewReader(out))
		lines.Buffer(make([]byte, 0, 1<<16), 1<<24)
		for lines.Scan() {
			path, rest, _ := strings.Cut(lines.Text(), "\t")
			dir, imported, _ := strings.Cut(rest, "\t")
			if path == "" || dir == "" {
				continue
			}
			g.dirs[path], g.imports[path] = dir, strings.Fields(imported)
			listed = append(listed, path)
		}
		if err := lines.Err(); err != nil {
			return nil, fmt.Errorf("checker: read go list: %w", err)
		}
		g.listed = append(g.listed, listed)
	}
	e.graph = g
	return g, nil
}

// affected returns the import paths of the packages that a change of the files at full, absolute
// paths, can break: each package in the directory of a file, and each package that imports one
// of them, directly or through another package of the workspace. A test that imports a package
// is part of the package that it tests.
func (g *graph) affected(full []string) []string {
	changed := map[string]bool{}
	for _, one := range full {
		changed[filepath.Dir(one)] = true
	}
	importers := map[string][]string{}
	for path, imported := range g.imports {
		for _, one := range imported {
			importers[one] = append(importers[one], path)
		}
	}
	reached := map[string]bool{}
	var queue []string
	for path, dir := range g.dirs {
		if changed[dir] {
			reached[path] = true
			queue = append(queue, path)
		}
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
	return slices.Sorted(maps.Keys(reached))
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
