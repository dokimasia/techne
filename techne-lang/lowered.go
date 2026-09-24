// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Lines are zero-based line numbers of files, by path.
type Lines map[source.Path]map[int]bool

// Spanned returns the lines from the start line to the end line of each span. A span that ends
// before its start line covers its start line.
func Spanned(spans ...source.Span) Lines {
	out := Lines{}
	for _, s := range spans {
		if out[s.Path] == nil {
			out[s.Path] = map[int]bool{}
		}
		for n := s.Start.Line; n <= max(s.End.Line, s.Start.Line); n++ {
			out[s.Path][n] = true
		}
	}
	return out
}

// Hides reports whether an error on a line can hide a use of the name that an answer is about.
// p is the file of the error, line is its zero-based line, and text is the line.
type Hides func(p source.Path, line int, text string) bool

// Writing returns the rule of an answer about name whose own sites are on the lines of covered.
// An error hides a use when it is on a line outside covered that contains name as a word, by
// [Worded]. A type checker binds every name that it can in a project with errors. A use that it
// cannot bind is on a line that it reports. An empty name returns [Everywhere].
func Writing(name string, covered Lines) Hides {
	if name == "" {
		return Everywhere
	}
	return func(p source.Path, line int, text string) bool {
		return !covered[p][line] && Worded(text, name) >= 0
	}
}

// Binding returns the rule of a definition of the name on line of the file at p. An error on
// that line can bind the name to the wrong declaration. When found is false, Binding returns
// [Everywhere], because an error anywhere in the project can leave the name unbound.
func Binding(p source.Path, line int, found bool) Hides {
	if !found {
		return Everywhere
	}
	return func(q source.Path, at int, _ string) bool { return q == p && at == line }
}

// Everywhere is the rule of an answer that an error anywhere in its project can make wrong,
// such as an empty definition.
func Everywhere(source.Path, int, string) bool { return true }

// Nowhere is the rule of a plan that rewrites no reference, such as an extraction. The gate of
// such a plan judges its result. No error hides a use from it.
func Nowhere(source.Path, int, string) bool { return false }

// Lowered returns [trust.Indexed] and a [trust.CaveatBuildBroken] caveat when an error of errors
// is on a line that risky reports, and [trust.None] and no caveat otherwise.
//
// errors are the zero-based lines of the errors of one project, by path. read returns the lines
// of a file. When read fails on a file, every line of the file with an error counts as a line
// that risky reports. reporter is the subject of the note of the caveat, such as "the server".
// The note lists the first five such lines in path and line order, with the number of the rest.
// The paths of the caveat are the files of those lines.
func Lowered(
	errors map[source.Path][]int,
	read func(p source.Path) (func(line int) string, error),
	risky Hides,
	reporter string,
) (trust.Fidelity, []trust.Caveat) {
	var sites []string
	var paths []source.Path
	for _, p := range slices.Sorted(maps.Keys(errors)) {
		line, err := read(p)
		before := len(sites)
		for _, n := range slices.Compact(slices.Sorted(slices.Values(errors[p]))) {
			if err != nil || risky(p, n, line(n)) {
				sites = append(sites, fmt.Sprintf("%s:%d", p, n+1))
			}
		}
		if len(sites) > before {
			paths = append(paths, p)
		}
	}
	if len(sites) == 0 {
		return trust.None, nil
	}
	return trust.Indexed, []trust.Caveat{{
		Code: trust.CaveatBuildBroken,
		Note: reporter + " reports an error that can hide a use at " + listed(sites) + ". Names " +
			"are bound where " + reporter + " could bind them and matched by text elsewhere",
		Paths: paths,
	}}
}

// listing is the number of lines that the note of a [trust.CaveatBuildBroken] caveat lists.
const listing = 5

// listed returns the first [listing] sites joined for a note, with the number of the rest.
func listed(sites []string) string {
	if len(sites) <= listing {
		return strings.Join(sites, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(sites[:listing], ", "), len(sites)-listing)
}
