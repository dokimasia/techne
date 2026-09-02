// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// Check reports what is wrong with content the workspace does not hold.
//
// A line that is not one of this language's four shapes, which is what a
// change breaks when it writes over the wrong bytes.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	mine := make([]source.Path, 0, len(files))
	for p := range files {
		if slices.Contains(e.declared.Extensions, path.Ext(string(p))) {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: nothing here is %s", engine.ErrDecline, e.declared.Language)
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
				Message: fmt.Sprintf("a line is %s, %s, %s or blank",
					documents, refers, strings.Join(Kinds(), ", ")),
				Span:    at,
				Source:  e.Name(),
				Snippet: quoted(files[p], at),
			}})
		}
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: e.coverage}, nil
}

// Verify reports what this language's own gate says about a scope.
//
// A use naming nothing is the one thing that parses and is still wrong,
// which is what a build gate catches and a parse gate does not. There is
// no obvious change that fixes it — which of the declarations in scope
// was meant is not something the engine knows — so the finding carries
// none, and that is the honest answer rather than a guess.
func (e *Engine) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	if len(suites) > 0 && !slices.Contains(suites, Suite) {
		return engine.Result[edit.Finding]{Completeness: e.coverage}, nil
	}

	held, err := e.read(ctx, req)
	if err != nil {
		return engine.Result[edit.Finding]{}, err
	}

	declared := map[string]bool{}
	for _, one := range held.symbols {
		declared[one.Name] = true
	}

	var out []edit.Finding
	for _, p := range slices.Sorted(maps.Keys(held.lines)) {
		for _, one := range held.lines[p] {
			if one.Uses == "" || declared[one.Uses] {
				continue
			}
			out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     "unresolved",
				Message:  fmt.Sprintf("nothing in scope is called %q", one.Uses),
				Span:     one.At,
				Source:   e.Name(),
				Snippet:  text(one),
			}})
		}
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: e.coverage}, nil
}

// Suite is the one thing this language runs. A caller naming any other
// gets nothing, which is what a language whose linter is not installed
// answers.
const Suite = "resolve"

// quoted is the text a span covers, for a reader with no file.
func quoted(content []byte, at source.Span) string {
	from, to := at.Start.Offset, at.End.Offset
	if from < 0 || to > len(content) || from > to {
		return ""
	}
	return strings.TrimSpace(string(content[from:to]))
}
