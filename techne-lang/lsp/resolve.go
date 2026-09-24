// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"

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
// language the result is skipped. A directory that contains files of the language returns
// [engine.ErrDecline], because a position belongs to one file.
//
// A location contains no name and no kind. Resolve reads the declarations of each file that a
// location names and returns the innermost declaration at the location. An error on the line of
// the position lowers the answer, and so does any error of the project when the answer is
// empty. An empty answer is partial, because a server binds nothing inside a macro, for
// dynamic dispatch and for a name that nothing declares alike. A server that does not answer
// within [Server.Answering] returns [engine.ErrDecline].
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	out, err := e.resolving(ctx, req, at)
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
	ctx, done := e.answered(ctx)
	defer done()
	if !provides(held.capable.DefinitionProvider) {
		return engine.Result[sema.Symbol]{}, e.unsupported("textDocument/definition")
	}
	ready := e.settle(ctx, held)

	answered, err := held.asks.Definition(ctx, &protocol.DefinitionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     doc.mark(at),
	})
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("lsp: %s: definition in %s: %w",
			e.server.Name, doc.path, err)
	}

	found := newFinder(e, held)
	var out []sema.Symbol
	for _, one := range definitions(answered) {
		declared, known, err := found.at(ctx, e.pathOf(one.URI), one.Range.Start)
		if err != nil {
			return engine.Result[sema.Symbol]{}, err
		}
		if known {
			out = append(out, declared)
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

// unbound is the caveat on an empty definition, which comes with [trust.ScopePartial]. An empty
// definition is no evidence that nothing declares the name.
var unbound = trust.Caveat{
	Code: trust.CaveatDynamic,
	Note: "the server binds nothing at this position, as a server does inside a macro, for " +
		"dynamic dispatch and for a name that nothing declares",
}

// pointed opens the file that the scope of req names, and returns the running server and the
// document. It reports true when the scope contains no file of the language. It returns
// [engine.ErrDecline] for a directory that contains files of the language and for a server
// that does not start.
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
			engine.ErrDecline, e.server.Name, req.Scope)
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
