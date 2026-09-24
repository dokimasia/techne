// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"os"
	"time"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/uri"
)

// dynamic is the caveat on every answer of an [Engine].
var dynamic = trust.Caveat{
	Code: trust.CaveatDynamic,
	Note: "the type checker of a language server does not follow reflection, " +
		"dispatch by string or struct tags",
}

// warming is the caveat on an answer from a server that had not settled when it was asked. It
// comes with [trust.ScopePartial].
var warming = trust.Caveat{
	Code: trust.CaveatIndexWarming,
	Note: "the server was still loading the workspace, so a declaration it did not " +
		"return may exist",
}

// unresolved is the caveat on an empty answer from a server that has not shown a view of the
// file. It comes with [trust.ScopePartial]. A server that analysed the file, found nothing and
// published nothing gets it too, and its answer is reported as partial.
var unresolved = trust.Caveat{
	Code: trust.CaveatIndexWarming,
	Note: "no diagnostics show that the server analysed this file, so an empty answer " +
		"may come from a file it did not analyse",
}

// unread returns the caveat that lists the files that the server was not given, because each
// is larger than [lang.Largest] or excluded by a .gitignore file, or nil for no file.
func unread(paths []source.Path) []trust.Caveat {
	if len(paths) == 0 {
		return nil
	}
	return []trust.Caveat{{
		Code:  trust.CaveatUnread,
		Note:  "the server was not given these files: each is larger than an engine reads or excluded by .gitignore",
		Paths: paths,
	}}
}

// settled returns the completeness and caveats of an answer, given whether the server settled
// before it was asked.
func settled(ready bool) (trust.Completeness, []trust.Caveat) {
	if ready {
		return trust.ScopeTotal, []trust.Caveat{dynamic}
	}
	return trust.ScopePartial, []trust.Caveat{dynamic, warming}
}

// bound returns the completeness, the lowered tier and the caveats of an answer that binds
// names in scope, given whether the server settled before it was asked. risky decides which
// errors of the project of scope lower the answer.
func (e *Engine) bound(
	held *session,
	scope source.Path,
	ready bool,
	risky lang.Hides,
) (trust.Completeness, trust.Fidelity, []trust.Caveat) {
	covered, caveats := settled(ready)
	reaches, why := e.lowered(held, e.project(scope), risky)
	return covered, reaches, append(caveats, why...)
}

// project returns the directory of the nearest manifest of the language above scope, from
// [lang.ProjectOf].
func (e *Engine) project(scope source.Path) source.Path {
	return lang.ProjectOf(os.DirFS(e.root), scope, e.declared.Manifests)
}

// lowered returns the tier and the caveats of [lang.Lowered] over the errors that the server
// has reported for the files in the project within.
func (e *Engine) lowered(held *session, within source.Path, risky lang.Hides) (trust.Fidelity, []trust.Caveat) {
	return lang.Lowered(held.reports.errors(within, e.pathOf), e.lines, risky, "the server")
}

// lines returns the lines of the file at p, as [lang.Lowered] reads them.
func (e *Engine) lines(p source.Path) (func(line int) string, error) {
	doc, err := e.read(p)
	if err != nil {
		return nil, err
	}
	return func(n int) string { return doc.line(uint32(n)) }, nil
}

// reached returns the completeness, the lowered tier and the caveats of a plan for the file
// at p. A plan without edits from a server that has not shown a view of p is partial, because
// an empty plan does not tell a file that nothing uses from a file that the server did not
// analyse. shown reports whether the server computed any edit. risky decides which errors
// lower the plan.
func (e *Engine) reached(
	ctx context.Context,
	held *session,
	p source.Path,
	shown, ready bool,
	risky lang.Hides,
) (trust.Completeness, trust.Fidelity, []trust.Caveat) {
	covered, reaches, caveats := e.bound(held, p, ready, risky)
	if covered == trust.ScopeTotal && !shown && !e.analysed(ctx, held, p) {
		return trust.ScopePartial, reaches, append(caveats, unresolved)
	}
	return covered, reaches, caveats
}

// analysed reports whether the server has shown a view of the file at p.
//
// A server with pull diagnostics has a view when it returns a full report for p. A server
// without them has a view once it publishes diagnostics for p, which analysed waits
// [reporting] for. A server that refuses the request has no view. metals publishes nothing for
// a file without errors, so analysed reports false for every clean Scala file.
func (e *Engine) analysed(ctx context.Context, held *session, p source.Path) bool {
	if held.capable.DiagnosticProvider == nil {
		_, said := held.reports.wait(ctx, uri.File(e.fullPath(p)), time.Now().Add(reporting))
		return said
	}
	_, full, err := e.pull(ctx, held, p)
	return err == nil && full
}
