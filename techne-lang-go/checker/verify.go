// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// removed is the content that a load reads for a file that a change deletes. No build
// satisfies its build constraint, so the file is in no package, as a deleted file is.
var removed = []byte("//go:build ignore && !ignore\n\npackage removed\n")

// Verify returns the errors that the type checker reports for the Go files in the scope of req,
// and for its test files only when req includes tests.
//
// The type checker runs one analysis, and a caveat lists the suites that Verify does not run.
// Verify returns a skipped result for a scope without a Go file. A module in the scope that
// fails to load makes the answer partial, and a caveat lists it with its error.
func (e *Engine) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	scope := scoped(req.Scope)
	w, err := e.walk()
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !w.claims(scope) {
		return engine.Result[edit.Finding]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}
	v, err := e.current(ctx, w)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	within := map[source.Path]bool{}
	for _, p := range w.files {
		if lang.Within(p, scope) && (req.Tests || !e.declared.IsTest(string(p))) {
			within[p] = true
		}
	}
	covered, missing := v.partial(scope)
	return engine.Result[edit.Finding]{
		Items:        e.faults(v, sources{}, func(p source.Path) bool { return within[p] }),
		Completeness: covered,
		Caveats:      slices.Concat(missing, reasons(suites)),
	}, nil
}

// Check returns the errors that the type checker reports for the workspace with the content of
// files in place of the content on disk. A file with nil content is one that the change
// deletes.
//
// Check type-checks every package of the workspace, so the errors include those that a change
// causes in the packages that import a changed package. When every file of files equals its
// content on disk, the cached view is the answer. Check declines when files contain no Go file,
// and when a package of the workspace does not compile a Go file of files, so that the next
// engine checks the change. A module that fails to load adds a caveat, because the packages in
// it that import a changed package are not checked.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	overlay := map[string][]byte{}
	var written []source.Path
	unchanged := true
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if !lang.Claims(string(p), e.declared.Extensions) {
			continue
		}
		full := e.fullPath(p)
		content := files[p]
		if content == nil {
			overlay[full], unchanged = removed, false
			continue
		}
		overlay[full] = content
		written = append(written, p)
		if disk, err := os.ReadFile(full); err != nil || !bytes.Equal(disk, content) {
			unchanged = false
		}
	}
	if len(overlay) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: checker: the change contains no %s file", engine.ErrDecline, e.declared.Language)
	}

	w, err := e.walk()
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	var v *view
	if unchanged {
		v, err = e.current(ctx, w)
	} else {
		v, err = e.load(ctx, w.modules, overlay)
	}
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	for _, p := range written {
		if v.compiles(e.fullPath(p)) {
			continue
		}
		reason := ""
		if covered, missing := v.partial(p); covered != trust.ScopeTotal {
			reason = ", and " + missing[0].Note
		}
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: checker: no loaded package compiles %s%s",
			engine.ErrDecline, p, reason)
	}

	var caveats []trust.Caveat
	if len(v.unloaded) > 0 {
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatDependents,
			Note: "the type checker could not load these modules and did not check their packages: " +
				failures(v.unloaded),
			Paths: slices.Sorted(maps.Keys(v.unloaded)),
		})
	}
	return engine.Result[edit.Finding]{
		Items:        e.faults(v, overlay, func(source.Path) bool { return true }),
		Completeness: trust.ScopeTotal,
		Caveats:      caveats,
	}, nil
}

// faults returns the errors of the packages of v whose file passes wanted, once each, sorted by
// path, offset and message. A package and its test variant report an error of a
// file that both compile, and faults returns it once. The offset and the source line of each
// error come from files, which reads a file from disk when it does not contain it.
func (e *Engine) faults(v *view, files sources, wanted func(source.Path) bool) []edit.Finding {
	seen := map[string]bool{}
	var out []edit.Finding
	for _, pkg := range v.all() {
		for _, held := range pkg.Errors {
			file, line, column := placed(held.Pos)
			p := e.pathOf(file)
			if file == "" || line < 1 || !lang.Within(p, engine.Root) || !wanted(p) {
				continue
			}
			message := strings.TrimSpace(held.Msg)
			key := fmt.Sprintf("%s:%d:%d:%s", p, line, column, message)
			if seen[key] {
				continue
			}
			seen[key] = true

			content := files.read(file)
			at := source.Position{Line: line - 1, Column: max(column-1, 0)}
			at.Offset = byteAt(content, at.Line, at.Column)
			out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     "compile",
				Message:  message,
				Span:     source.Span{Path: p, Start: at, End: at},
				Source:   "go/types",
				Snippet:  strings.TrimSpace(lang.LineAt(content, at.Offset)),
			}})
		}
	}
	slices.SortFunc(out, func(a, b edit.Finding) int {
		return cmp.Or(
			strings.Compare(string(a.Diagnostic.Span.Path), string(b.Diagnostic.Span.Path)),
			cmp.Compare(a.Diagnostic.Span.Start.Offset, b.Diagnostic.Span.Start.Offset),
			strings.Compare(a.Diagnostic.Message, b.Diagnostic.Message),
		)
	})
	return out
}

// placed returns the file, the line and the column of a position of the loader, which has the
// form file:line:column or file:line, or is - or empty for none. It reads the numbers from the
// end, because a file can contain a colon, as a Windows path does. It returns an empty file for
// no position.
func placed(pos string) (string, int, int) {
	if pos == "" || pos == "-" {
		return "", 0, 0
	}
	file := pos
	var numbers []int
	for len(numbers) < 2 {
		at := strings.LastIndexByte(file, ':')
		if at < 0 {
			break
		}
		n, err := strconv.Atoi(file[at+1:])
		if err != nil {
			break
		}
		numbers, file = append(numbers, n), file[:at]
	}
	switch len(numbers) {
	case 2:
		return file, numbers[1], numbers[0]
	case 1:
		return file, numbers[0], 0
	}
	return file, 0, 0
}

// reasons returns the caveat that lists the suites that a caller asked for, which the type
// checker does not run, or nil for none.
func reasons(suites []string) []trust.Caveat {
	if len(suites) == 0 {
		return nil
	}
	return []trust.Caveat{{
		Code: trust.CaveatUnsupported,
		Note: "the type checker runs its own analysis and none of these suites: " + strings.Join(suites, ", "),
	}}
}
