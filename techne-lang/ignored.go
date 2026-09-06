// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"bufio"
	"io/fs"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// ignores is what a workspace says is not its own source, read from the
// .gitignore files in it.
//
// # Why version control decides this
//
// [Vendored] names the directories every ecosystem puts dependencies in.
// It cannot name build output, because dist and build are generated in
// one project and hand-written in the next, and a fixed list that
// guessed would lose somebody's source.
//
// A repository has already answered the question. Measured over one
// TypeScript monorepo: techne walked 1,725 files where the project's own
// source is 1,045, and the 680 it added were a storybook bundle and
// fifty-four dist directories that .gitignore names on lines 11 and 40.
// Among them were minified bundles of three megabytes, which is what a
// search over the workspace was spending its time on.
//
// # What it implements, and what it does not
//
// The .gitignore file in each directory, applying to that directory and
// below, with git's own precedence: the last pattern that matches wins,
// and a file deeper down overrides one above it. Negation, anchoring, a
// trailing slash for directories only, and the * ? [] and ** wildcards.
//
// Not the global excludes file and not .git/info/exclude. Both live
// outside the workspace, so honouring them would make one repository
// answer differently on two machines.
//
// Not the index either. Git does not ignore a file it already tracks, so
// a file force-added under an ignored path is source to git and is
// skipped here. Reading the index means running git, which is a
// subprocess per walk and per language; the same trade is what ripgrep
// and fd make.
type ignores struct {
	// held is the rules of each .gitignore, by the directory it sits in.
	// A path is judged by every ancestor's rules in turn, so a deeper
	// file has the last word.
	held map[string][]pattern
}

// pattern is one line of a .gitignore.
type pattern struct {
	// negate re-includes what an earlier pattern excluded.
	negate bool
	// dirOnly matches directories alone, which is what a trailing slash
	// means.
	dirOnly bool
	// anchored fixes the match to the directory the file sits in, which
	// is what a slash anywhere but the end means.
	anchored bool
	parts    []string
}

// reading collects the .gitignore in one directory, if it has one.
//
// Called as a walk enters each directory, so a file is read once and
// only where a walk goes.
func (i *ignores) reading(fsys fs.FS, dir string) {
	content, err := fs.ReadFile(fsys, path.Join(dir, ".gitignore"))
	if err != nil {
		return
	}
	if held := parsed(string(content)); len(held) > 0 {
		if i.held == nil {
			i.held = map[string][]pattern{}
		}
		i.held[dir] = held
	}
}

// skips reports whether the workspace says a path is not its source.
//
// Every ancestor's rules apply, shallowest first, and the last pattern
// that matches decides. That is git's own precedence and it is what
// makes a negation in a nested file able to re-include something the
// root excluded.
func (i *ignores) skips(p string, isDir bool) bool {
	if len(i.held) == 0 {
		return false
	}
	ignored := false
	for _, dir := range ancestors(p) {
		rules, held := i.held[dir]
		if !held {
			continue
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(p, dir), "/")
		if dir == "." {
			rest = p
		}
		for _, one := range rules {
			if one.matches(rest, isDir) {
				ignored = !one.negate
			}
		}
	}
	return ignored
}

// ancestors are the directories a path sits under, shallowest first,
// including the workspace root.
func ancestors(p string) []string {
	out := []string{"."}
	held := path.Dir(p)
	if held == "." {
		return out
	}

	var stack []string
	for held != "." && held != "/" && held != "" {
		stack = append(stack, held)
		held = path.Dir(held)
	}
	for _, one := range slices.Backward(stack) {
		out = append(out, one)
	}
	return out
}

// parsed reads the patterns out of one .gitignore.
func parsed(content string) []pattern {
	var out []pattern
	lines := bufio.NewScanner(strings.NewReader(content))
	for lines.Scan() {
		if held, ok := rule(lines.Text()); ok {
			out = append(out, held)
		}
	}
	return out
}

// rule reads one line, and reports whether it is a pattern at all.
//
// A blank line and a comment are neither. Trailing whitespace is not
// part of a pattern unless it was escaped, which is git's rule and the
// reason a line ending in a stray space does not silently match nothing.
func rule(line string) (pattern, bool) {
	held := strings.TrimRight(line, " \t")
	if kept, escaped := strings.CutSuffix(line, "\\ "); escaped {
		held = kept + " "
	}
	if held == "" || strings.HasPrefix(held, "#") {
		return pattern{}, false
	}

	out := pattern{}
	if strings.HasPrefix(held, "!") {
		out.negate, held = true, held[1:]
	}
	if kept, only := strings.CutSuffix(held, "/"); only {
		out.dirOnly, held = true, kept
	}
	if held == "" {
		return pattern{}, false
	}
	if strings.HasPrefix(held, "/") {
		out.anchored, held = true, held[1:]
	} else if strings.Contains(held, "/") {
		// A slash anywhere but the end fixes the pattern to the
		// directory the file sits in. Without one it matches a name at
		// any depth, which is what makes "dist" cover every dist there
		// is.
		out.anchored = true
	}

	out.parts = strings.Split(held, "/")
	return out, len(out.parts) > 0
}

// matches reports whether a path relative to the pattern's own directory
// is one this pattern names.
func (p pattern) matches(rest string, isDir bool) bool {
	if rest == "" {
		return false
	}
	segments := strings.Split(rest, "/")

	if p.anchored {
		return p.covers(segments, 0, isDir)
	}
	// A pattern with no slash names something at any depth, so it is
	// tried from every segment. A directory-only pattern that matched
	// part way along matched a directory, whatever the path ends in.
	for at := range segments {
		if p.covers(segments, at, isDir) {
			return true
		}
	}
	return false
}

// covers matches the pattern's segments against the path's, from one
// starting point.
//
// A ** consumes any number of segments, including none. Everything else
// matches one segment through [path.Match], whose wildcards stop at a
// separator, which is what keeps * from crossing a directory.
func (p pattern) covers(segments []string, at int, isDir bool) bool {
	held, rest := p.parts, segments[at:]
	for len(held) > 0 {
		if held[0] == "**" {
			held = held[1:]
			if len(held) == 0 {
				// Everything below is named, and everything below a
				// directory is inside one.
				return true
			}
			for skip := range len(rest) + 1 {
				if p.tail(held, rest[skip:], isDir) {
					return true
				}
			}
			return false
		}
		if len(rest) == 0 {
			return false
		}
		if ok, err := path.Match(held[0], rest[0]); err != nil || !ok {
			return false
		}
		held, rest = held[1:], rest[1:]
	}
	// The pattern ran out. It named this path if it ended here, or named
	// a directory that holds it — which is why an ignored directory
	// takes everything under it.
	return !p.dirOnly || isDir || len(rest) > 0
}

// tail is covers for what is left after a **.
func (p pattern) tail(held, rest []string, isDir bool) bool {
	return pattern{parts: held, dirOnly: p.dirOnly, anchored: true}.
		covers(append([]string{}, rest...), 0, isDir)
}

// GeneratedError is why a scope was not read: the workspace itself says it
// holds output rather than source.
//
// An error rather than an empty answer, because the two are different
// facts and a caller acts on them differently. Told nothing is there, it
// concludes the directory is empty; told the workspace calls it
// generated, it knows to look at what wrote it.
type GeneratedError struct{ Scope source.Path }

func (g GeneratedError) Error() string {
	return "lang: " + string(g.Scope) + " is generated: the workspace's .gitignore names it, " +
		"so it holds output rather than source"
}
