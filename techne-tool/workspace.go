// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// WorkspaceInput is the input of the workspace tool.
type WorkspaceInput struct {
	Scope     string `json:"scope,omitempty"      jsonschema:"directory to map, the root by default"`
	Language  string `json:"language,omitempty"   jsonschema:"a language to ask instead of the scope's"`
	Private   bool   `json:"private,omitempty"    jsonschema:"count unexported declarations"`
	Tests     bool   `json:"tests,omitempty"      jsonschema:"include test files"`
	MaxTokens int    `json:"max_tokens,omitempty" jsonschema:"answer ceiling in tokens, 6000 by default"`
}

// Units is the output of the workspace tool: the units of a directory, such as the packages of
// Go, in the order of their paths.
type Units struct {
	Scope      Scope      `json:"scope"`
	Items      []Unit     `json:"items"`
	Provenance Provenance `json:"provenance"`
	Error      *Failure   `json:"error,omitempty"`
}

// Unit is one unit of a language: its path, its language, the number of its files and of its
// declarations, and the first sentence of its documentation.
type Unit struct {
	Unit     string `json:"unit"`
	Language string `json:"language"`
	Files    int    `json:"files"`
	// Declarations counts the declarations at the top level of the files of the unit that are
	// visible outside it, or all of them for an input with private.
	Declarations int `json:"declarations"`
	// Summary is the first sentence of the documentation of the package or the module that
	// the unit declares, or empty for none.
	Summary string `json:"summary,omitempty"`
}

// Failed reports whether the output has an Error.
func (u Units) Failed() bool { return u.Error != nil }

// Render returns the units as text: a heading with the count, one line per unit with its
// language, its files, its declarations and its summary, and the evidence.
func (u Units) Render() string {
	var b strings.Builder
	if u.Error != nil {
		fmt.Fprintf(&b, "%s: %s\n", u.Error.Code, u.Error.Reason)
		return b.String()
	}
	b.WriteString(u.heading(len(u.Items)))
	b.WriteString("\n\n")
	if len(u.Items) == 0 {
		b.WriteString("no unit\n")
	}
	var widths columns
	for _, one := range u.Items {
		widths = widths.fit(one)
	}
	for _, one := range u.Items {
		b.WriteString(one.render(widths))
	}
	b.WriteString("\n")
	b.WriteString(evidence(u.Provenance, len(u.Items) == 0))
	return b.String()
}

// heading returns the first line of the render of u with n units.
func (u Units) heading(n int) string {
	return fmt.Sprintf("%s — %s", cmp.Or(u.Scope.Directory, string(engine.Root)), plural(n, "unit", "units"))
}

// columns are the widths of the columns of the render of units: the path, the language, and
// the digits of the counts of files and declarations.
type columns struct{ unit, language, files, declarations int }

// fit returns c widened to the columns of u.
func (c columns) fit(u Unit) columns {
	return columns{
		unit:         max(c.unit, len(u.Unit)),
		language:     max(c.language, len(u.Language)),
		files:        max(c.files, len(strconv.Itoa(u.Files))),
		declarations: max(c.declarations, len(strconv.Itoa(u.Declarations))),
	}
}

// render returns the line of u in the columns c, with its summary last.
func (u Unit) render(c columns) string {
	line := fmt.Sprintf("%-*s  %-*s  %*d %-5s  %*d %s", c.unit, u.Unit, c.language, u.Language,
		c.files, u.Files, noun(u.Files, "file", "files"),
		c.declarations, u.Declarations, noun(u.Declarations, "declaration", "declarations"))
	if u.Summary != "" {
		line = strings.TrimRight(line, " ") + "  " + u.Summary
	}
	return strings.TrimRight(line, " ") + "\n"
}

// Workspace returns the tool that maps the units of a directory. It outlines the directory,
// groups the declarations by the language and the unit of their IDs, and fits the output to the
// budget with the first units whose render fits, and at least one.
func Workspace(reads Outliner) (Tool, error) {
	return New("workspace", workspaceDescription,
		func(ctx context.Context, in WorkspaceInput) (Units, error) {
			scope, failure := relative(in.Scope)
			if failure == nil {
				failure = unfiled(scope)
			}
			if failure != nil {
				return Units{
					Scope: Scope{Language: in.Language}, Items: []Unit{}, Provenance: unserved(), Error: failure,
				}, nil
			}
			answered, err := reads.Outline(ctx, engine.Request{
				Scope:    scope,
				Language: source.Language(in.Language),
				Tests:    in.Tests,
			})
			if err != nil {
				return Units{}, err
			}
			out := Units{
				Scope:      Scope{Language: in.Language, Directory: string(scope)},
				Items:      unitsOf(answered.Items, in.Private),
				Provenance: provenance(answered.Provenance),
			}
			switch answered.Status {
			case trust.Unsupported, trust.Refused:
				out.Error = &Failure{Code: answered.Status.String(), Reason: reasonFrom(out.Provenance.Caveats)}
			case trust.Unset, trust.OK, trust.Degraded, trust.Partial:
			}
			return out.fitted(Budget{MaxTokens: in.MaxTokens}), nil
		})
}

// unfiled returns a refusal for a scope that [names] reports as a file, because the workspace
// tool maps a directory.
func unfiled(scope source.Path) *Failure {
	if !names(scope) {
		return nil
	}
	return &Failure{
		Code: trust.Refused.String(),
		Reason: fmt.Sprintf(
			"scope %q names a file, and the workspace tool maps a directory: outline lists a file", scope),
	}
}

// unitsOf returns the units of items, in the order of their paths and then their languages.
// A unit counts the files and the declarations at the top level of [Declared] at [Names], and
// only the declarations visible outside it without private. Its summary is the first sentence
// of the first documentation of a package or a module that it declares.
func unitsOf(items []sema.Symbol, private bool) []Unit {
	type key struct{ unit, language string }
	grouped := map[key][]sema.Symbol{}
	for _, s := range items {
		k := key{unit: string(s.ID.Unit()), language: string(s.Language)}
		grouped[k] = append(grouped[k], s)
	}
	out := make([]Unit, 0, len(grouped))
	for k, symbols := range grouped {
		one := Unit{Unit: k.unit, Language: k.language}
		files := map[source.Path]bool{}
		for _, s := range symbols {
			files[s.Span.Path] = true
		}
		one.Files, one.Summary = len(files), unitSummary(symbols)
		one.Declarations = len(Narrow{Private: private}.Apply(Declared(symbols, Names, 0)))
		out = append(out, one)
	}
	slices.SortFunc(out, byPath)
	return out
}

// byPath orders units by their paths, and units of one path by their languages.
func byPath(a, b Unit) int {
	return cmp.Or(cmp.Compare(a.Unit, b.Unit), cmp.Compare(a.Language, b.Language))
}

// fitted returns u when its render fits b. Otherwise it returns the most units, at least one,
// whose render fits, taken by their declarations, the most first, and listed in the order of
// their paths, with a truncation caveat. The units with the most declarations are the ones
// that contain the code of a large repository, where the first units by path are often its
// build files.
func (u Units) fitted(b Budget) Units {
	if u.Error != nil || len(u.Items) == 0 {
		return u
	}
	limit := (cmp.Or(b.MaxTokens, DefaultMaxTokens)+1)*bytesPerToken - 1
	if len(u.Render()) <= limit {
		return u
	}
	ranked := slices.Clone(u.Items)
	slices.SortStableFunc(ranked, func(a, b Unit) int { return cmp.Compare(b.Declarations, a.Declarations) })
	chosen := func(k int) Units {
		out := u
		out.Items = slices.Clone(ranked[:k])
		slices.SortFunc(out.Items, byPath)
		return out
	}
	k := longest(len(ranked), func(k int) bool { return len(chosen(k).Render()) <= limit })
	out := chosen(k)
	out.Provenance.Caveats = append(append([]Caveat(nil), u.Provenance.Caveats...), Caveat{
		Code: string(trust.CaveatTruncated),
		Note: fmt.Sprintf("%d of %d units returned within the token budget, those with the most declarations",
			k, len(u.Items)),
	})
	return out
}

const workspaceDescription = "PREFER OVER ls and find for learning what a workspace contains. " +
	"It lists each unit, such as a Go package, with its language, its files, its declarations " +
	"and the first sentence of its documentation."
