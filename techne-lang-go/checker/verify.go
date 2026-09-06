// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/packages"
)

// Verify reports what the compiler says about a scope.
//
// The type checker's own errors, which is what "does this build" means
// for Go. It runs no linter and no test suite: a caller naming suites is
// answered from the one analysis there is, and the caveat says so.
func (e *Engine) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	paths, err := lang.FilesIn(os.DirFS(e.root), req.Scope, e.declared.Extensions)
	if err != nil {
		return engine.Result[edit.Finding]{}, err
	}
	if len(paths) == 0 {
		return engine.Result[edit.Finding]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	v, err := e.current(ctx)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	within := map[source.Path]bool{}
	for _, p := range paths {
		if req.Tests || !e.declared.IsTest(string(p)) {
			within[p] = true
		}
	}

	out := e.faults(v, func(p source.Path) bool { return within[p] })
	return engine.Result[edit.Finding]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Caveats:      reasons(suites),
	}, nil
}

// Check reports what the compiler makes of content the workspace does
// not hold.
//
// The loader reads the named paths from memory and everything else from
// disk, so what is type-checked is the workspace as the change would
// leave it. That is what makes a gate a compiler's answer rather than a
// parser's: a rename onto a name already taken parses and does not
// build, and nothing but a type checker tells the two apart.
//
// Nothing is cached from it. The view it produces describes a workspace
// that does not exist.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	overlay := map[string][]byte{}
	judged := map[source.Path]bool{}
	for p, content := range files {
		if filepath.Ext(string(p)) != ".go" {
			continue
		}
		judged[p] = true
		// A path with no content is one the change takes away, and an
		// overlay says so with an empty file: the loader has no way to
		// unsee one, and an empty Go file is not a package member.
		overlay[filepath.Join(e.root, filepath.FromSlash(string(p)))] = content
	}
	if len(judged) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: nothing here is %s", engine.ErrDecline, e.declared.Language)
	}

	v, err := e.load(ctx, overlay)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	// Every package, not only the files handed over: a change that stops
	// something else compiling is the failure a gate exists to catch,
	// and a rename that leaves one caller behind breaks the caller's
	// file rather than the one that was edited.
	return engine.Result[edit.Finding]{
		Items:        e.faults(v, func(source.Path) bool { return true }),
		Completeness: trust.ScopeTotal,
	}, nil
}

// faults is what the type checker objected to, for the files a caller
// wants.
//
// Deduplicated by where they were reported: loading with tests
// type-checks a package twice, and an error in a file both variants
// compile is one error reported two ways.
func (e *Engine) faults(v *view, wanted func(source.Path) bool) []edit.Finding {
	seen := map[string]bool{}
	var out []edit.Finding

	for _, pkg := range v.pkgs {
		for _, held := range pkg.Errors {
			p, at := placed(held)
			held.Msg = strings.TrimSpace(held.Msg)
			if p == "" {
				continue
			}
			named := e.pathOf(p)
			if !inside(named) || !wanted(named) {
				continue
			}
			key := fmt.Sprintf("%s:%d:%d:%s", named, at.Line, at.Column, held.Msg)
			if seen[key] {
				continue
			}
			seen[key] = true

			out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
				// Everything the type checker reports stops the build,
				// and a list error is the go command refusing to name
				// the package at all, which stops it harder.
				Severity: diag.SeverityError,
				Code:     "compile",
				Message:  held.Msg,
				Span: source.Span{
					Path:  named,
					Start: source.Position{Line: at.Line - 1, Column: at.Column - 1},
				},
				Source: "go/types",
			}})
		}
	}
	return out
}

// placed reads the file and position out of a loader error, which
// carries them as text rather than as a position.
func placed(held packages.Error) (string, struct{ Line, Column int }) {
	var at struct{ Line, Column int }
	file, rest, cut := strings.Cut(held.Pos, ":")
	if !cut {
		return "", at
	}
	line, column, _ := strings.Cut(rest, ":")
	at.Line, at.Column = number(line), number(column)
	return file, at
}

// number reads a decimal, and zero for anything else.
func number(held string) int {
	out := 0
	for _, r := range held {
		if r < '0' || r > '9' {
			return out
		}
		out = out*10 + int(r-'0')
	}
	return out
}

// reasons states what this answer is not.
func reasons(suites []string) []trust.Caveat {
	if len(suites) == 0 {
		return nil
	}
	return []trust.Caveat{{
		Code: trust.CaveatUnsupported,
		Note: "the type checker has one analysis and no suites to choose between, " +
			"so every suite named was answered from the same one",
	}}
}
