// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Resolve returns the declarations that the name at a position denotes, from
// textDocument/definition. Two or more declarations mean that the name is ambiguous, such as
// a method of an interface with two or more implementations.
//
// The scope of req is the file that contains the position. For a scope without a file of the
// language the result is skipped. A directory that contains files of the language is refused
// with [engine.ErrRefuse], because a position belongs to one file, and so is a position outside
// its file, by the rule of [lang.Offset].
//
// A location contains no name and no kind. Resolve reads the declarations of each file that a
// location names, through the outline engine of the language when the engine has one, and
// returns the innermost declaration at the location, by the rule of [Engine.denoted], each
// declaration once. A server without pull diagnostics that returns no definition in a file it
// has published no report of is asked again after it publishes one, which Resolve waits
// [reporting] for. An error on the line of
// the position lowers the answer, and so does any error of the project when the answer is
// empty. An empty answer is partial, because a server returns no definition inside a macro,
// at a dynamic dispatch or for a name that nothing declares. A server that does not answer
// within [Server.Answering] returns [engine.ErrDecline]. A server that returns no definition
// and has stopped answering, as [Engine.silent] finds, is stopped, and Resolve asks a new
// server once. A new server that has stopped answering too returns [engine.ErrDecline].
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	out, err := e.resolving(ctx, req, at)
	if errors.Is(err, errStopped) {
		out, err = e.resolving(ctx, req, at)
	}
	return out, e.unanswered(ctx, err)
}

// resolving is [Engine.Resolve] before a missed deadline becomes a decline.
func (e *Engine) resolving(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	held, doc, skipped, err := e.pointed(ctx, req)
	if err != nil || skipped {
		return engine.Result[sema.Symbol]{Skipped: skipped, Completeness: trust.ScopeTotal}, err
	}
	defer e.reading()()
	if at.Offset <= 0 {
		if _, outside := lang.Offset(doc.path, doc.content, at.Line, at.Column); outside != nil {
			return engine.Result[sema.Symbol]{}, outside
		}
	}
	ctx, done := e.answered(ctx)
	defer done()
	if !provides(held.capable.DefinitionProvider) {
		return engine.Result[sema.Symbol]{}, e.unsupported("textDocument/definition")
	}
	ready := e.settle(ctx, held)

	asked := protocol.DefinitionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     doc.mark(at),
	}
	unseen := held.capable.DiagnosticProvider == nil && !held.reports.said(asked.TextDocument.URI)
	answered, err := held.asks.Definition(ctx, &asked)
	// typescript-language-server returns an empty definition in the first file of a project
	// that it opens while it loads the project, and publishes the diagnostics of the file after
	// the load. An empty definition in a file without a report is asked again once the report
	// arrives.
	if err == nil && unseen && len(definitions(answered)) == 0 && e.analysed(ctx, held, doc.path) {
		answered, err = held.asks.Definition(ctx, &asked)
	}
	if err != nil {
		return engine.Result[sema.Symbol]{}, replied(e.server.Name, "definition in "+string(doc.path), err)
	}

	found := newFinder(e, held)
	if len(definitions(answered)) == 0 && e.silent(ctx, held, found, doc.path) {
		e.discard(ctx, held)
		return engine.Result[sema.Symbol]{}, e.stopped(doc.path)
	}
	var out []sema.Symbol
	for _, one := range definitions(answered) {
		declared, err := e.denoted(ctx, held, found, one)
		if err != nil {
			return engine.Result[sema.Symbol]{}, err
		}
		for _, d := range declared {
			if !slices.ContainsFunc(out, func(o sema.Symbol) bool { return o.Span == d.Span }) {
				out = append(out, d)
			}
		}
	}

	risky := lang.Binding(doc.path, int(doc.mark(at).Line), len(out) > 0)
	covered, reaches, caveats := e.bound(held, req.Scope, ready, risky)
	if len(out) == 0 {
		covered, caveats = trust.ScopePartial, append(caveats, unbound)
	}
	return engine.Result[sema.Symbol]{
		Items:        out,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// denoted returns the innermost declaration at a location of a definition. A location that no
// declaration contains is asked for its own definition once, and denoted returns the
// declarations at the locations of that answer. typescript-language-server answers the
// definition of an imported name with the export { X } of its module and with a JSDoc tag that
// names X, and the definition of either is the declaration of X. A file at the location that
// [lang.Readable] refuses has no declaration.
func (e *Engine) denoted(
	ctx context.Context,
	held *session,
	found *finder,
	at protocol.Location,
) ([]sema.Symbol, error) {
	p := e.pathOf(at.URI)
	declared, known, err := found.at(ctx, p, at.Range.Start)
	switch {
	case err != nil:
		return nil, err
	case known:
		return []sema.Symbol{declared}, nil
	}
	if _, err = e.open(ctx, held, p); refused(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	answered, err := held.asks.Definition(ctx, &protocol.DefinitionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: at.URI},
		Position:     at.Range.Start,
	})
	if err != nil {
		return nil, replied(e.server.Name, "definition in "+string(p), err)
	}
	var out []sema.Symbol
	for _, next := range definitions(answered) {
		if next == at {
			continue
		}
		declared, known, err = found.at(ctx, e.pathOf(next.URI), next.Range.Start)
		if err != nil {
			return nil, err
		}
		if known {
			out = append(out, declared)
		}
	}
	return out, nil
}

// unbound is the caveat on an empty definition, which comes with [trust.ScopePartial]. An empty
// definition is no evidence that nothing declares the name.
var unbound = trust.Caveat{
	Code: trust.CaveatDynamic,
	Note: "the server binds nothing at this position, as a server does inside a macro, for " +
		"dynamic dispatch and for a name that nothing declares",
}

// pointed opens the file that the scope of req names, and returns the running server and the
// document. It reports true when the scope contains no file of the language. It returns
// [engine.ErrRefuse] for a directory that contains files of the language, and
// [engine.ErrDecline] for a server that does not start.
func (e *Engine) pointed(ctx context.Context, req engine.Request) (*session, document, bool, error) {
	if !lang.Claims(string(req.Scope), e.declared.Extensions) {
		files, err := e.walk(req)
		if err != nil {
			return nil, document{}, false, err
		}
		if len(files.Read) == 0 && len(files.Unread) == 0 {
			return nil, document{}, true, nil
		}
		return nil, document{}, false, fmt.Errorf("%w: %s: a position names a file, and %s is a directory",
			engine.ErrRefuse, e.server.Name, req.Scope)
	}

	held, err := e.running(ctx)
	if err != nil {
		return nil, document{}, false, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	doc, err := e.open(ctx, held, req.Scope)
	if err != nil {
		return nil, document{}, false, err
	}
	return held, doc, false, nil
}

// definitions returns the locations of a definition reply, which LSP 3.17 allows as one
// Location, a list of Location entries, or a list of LocationLink entries. The location of a
// link is its target selection range: the name of the declaration.
func definitions(answered protocol.DefinitionResult) []protocol.Location {
	switch reported := answered.(type) {
	case *protocol.Location:
		if reported == nil {
			return nil
		}
		return []protocol.Location{*reported}
	case protocol.LocationSlice:
		return reported
	case protocol.DefinitionLinkSlice:
		out := make([]protocol.Location, 0, len(reported))
		for _, one := range reported {
			out = append(out, protocol.Location{URI: one.TargetURI, Range: one.TargetSelectionRange})
		}
		return out
	}
	return nil
}
