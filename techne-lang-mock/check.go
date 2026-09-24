// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Suite is the name of the one check that [Engine.Verify] runs. It reports each use of a name
// that the workspace does not declare.
const Suite = "resolve"

// Check returns an error for each line of files that the language does not have, in path
// order. A file with nil content is one that a change deletes, and has no line to check. Check
// declines when files contain no file of the language, so that the next engine checks them.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	var mine []source.Path
	for p := range files {
		if lang.Claims(string(p), e.declared.Extensions) {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: mock: the change contains no %s file", engine.ErrDecline, e.declared.Language)
	}
	slices.Sort(mine)

	var out []edit.Finding
	for _, p := range mine {
		if err := ctx.Err(); err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		if files[p] == nil {
			continue
		}
		_, broken := Parse(p, files[p])
		for _, at := range broken {
			out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     "syntax",
				Message: fmt.Sprintf("a line is blank, a line of %s, a use or a declaration of %s",
					documents, strings.Join(Kinds(), ", ")),
				Span:    at,
				Source:  e.Name(),
				Snippet: quoted(files[p], at),
			}})
		}
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: e.coverage}, nil
}

// Verify returns an error for each use in the scope of req of a name that the workspace does
// not declare, and for the test files only when req includes tests. An error has no fix. A
// file of the workspace that is too large to read can declare the name of a use, so it makes
// the answer partial.
//
// The language has one check, [Suite], which Verify runs for every request. A caveat lists
// the other suites that req names. Verify returns a skipped result for a scope without a file
// of the language.
func (e *Engine) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	in, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[edit.Finding]{}, err
	}
	all, err := e.read(ctx, everywhere(req))
	if err != nil {
		return engine.Result[edit.Finding]{}, err
	}
	declared := map[string]bool{}
	for _, one := range all.symbols {
		declared[one.Name] = true
	}

	var out []edit.Finding
	for _, p := range slices.Sorted(maps.Keys(in.lines)) {
		for _, one := range in.lines[p] {
			if one.Uses == "" || declared[one.Uses] {
				continue
			}
			out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     "unresolved",
				Message:  fmt.Sprintf("no declaration of the workspace is named %s", one.Uses),
				Span:     one.At,
				Source:   e.Name(),
				Snippet:  text(one),
			}})
		}
	}

	answer := result(e, out, all, req.Scope)
	if others := slices.DeleteFunc(slices.Clone(suites), func(s string) bool { return s == Suite }); len(others) > 0 {
		answer.Caveats = append(answer.Caveats, trust.Caveat{
			Code: trust.CaveatUnsupported,
			Note: "the mock language runs its one check, " + Suite + ", and none of these suites: " +
				strings.Join(others, ", "),
		})
	}
	return answer, nil
}

// quoted returns the text that at covers in content, without the white space around it, or
// the empty string for a span outside content.
func quoted(content []byte, at source.Span) string {
	from, to := at.Start.Offset, at.End.Offset
	if from < 0 || to > len(content) || from > to {
		return ""
	}
	return strings.TrimSpace(string(content[from:to]))
}
