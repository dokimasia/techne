// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bytes"
	"maps"
	"os"
	"slices"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// dynamic is the caveat on every answer of an [Engine] that binds names.
var dynamic = trust.Caveat{
	Code: trust.CaveatDynamic,
	Note: "the Go type checker does not follow reflection, dispatch by string or struct tags",
}

// lowered returns the tier and the caveats of [lang.Lowered] over the errors of the program
// that type-checks the file at p. Under a go.work file that program is every module of the
// workspace, and otherwise it is the module of p. risky decides which errors lower the answer.
func (e *Engine) lowered(v *view, p source.Path, risky lang.Hides) (trust.Fidelity, []trust.Caveat) {
	return lang.Lowered(e.errors(v.program(p)), e.lines, risky, "the type checker")
}

// errors returns the zero-based lines of the errors of the packages of one program, by the
// workspace path of their file. An error outside the workspace or without a line is left out.
func (e *Engine) errors(one program) map[source.Path][]int {
	out := map[source.Path][]int{}
	for _, pkg := range one.pkgs {
		for _, held := range pkg.Errors {
			file, line, _ := placed(held.Pos)
			if p := e.pathOf(file); file != "" && line > 0 && lang.Within(p, engine.Root) {
				out[p] = append(out[p], line-1)
			}
		}
	}
	return out
}

// lines returns the lines of the file at p, as [lang.Lowered] reads them.
func (e *Engine) lines(p source.Path) (func(int) string, error) {
	content, err := os.ReadFile(e.fullPath(p))
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(content, []byte("\n"))
	return func(n int) string {
		if n < 0 || n >= len(lines) {
			return ""
		}
		return string(lines[n])
	}, nil
}

// partial returns the completeness of an answer about scope, and the caveat that lists the
// modules that overlap scope and failed to load, with the error of each. Without such a module
// it returns [trust.ScopeTotal] and no caveat.
func (v *view) partial(scope source.Path) (trust.Completeness, []trust.Caveat) {
	failed := map[source.Path]string{}
	for m, reason := range v.unloaded {
		if lang.Within(m, scope) || lang.Within(scope, m) {
			failed[m] = reason
		}
	}
	if len(failed) == 0 {
		return trust.ScopeTotal, nil
	}
	return trust.ScopePartial, []trust.Caveat{{
		Code:  trust.CaveatUnsupported,
		Note:  "the type checker could not load these modules: " + failures(failed),
		Paths: slices.Sorted(maps.Keys(failed)),
	}}
}
