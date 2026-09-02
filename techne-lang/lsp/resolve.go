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

// Resolve reports what the name at a position denotes.
//
// This is the question a server exists to answer. A parser reading the
// same file sees a name and no binding for it; a server has run the type
// checker and knows which declaration the name reaches, in this file or
// any other.
//
// More than one answer means the name is ambiguous — an interface method
// with several implementations, an overload set — and the caller
// chooses. It is not this engine's place to pick.
func (e *Engine) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Result[sema.Symbol], error) {
	held, doc, ok, err := e.pointed(ctx, req)
	if err != nil || !ok {
		return engine.Result[sema.Symbol]{Skipped: !ok, Completeness: trust.ScopeTotal}, err
	}

	answered, err := held.asks.Definition(ctx, &protocol.DefinitionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     doc.mark(at),
	})
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("lsp: %s: resolve %s: %w",
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

	return engine.Result[sema.Symbol]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Caveats:      []trust.Caveat{dynamic},
	}, nil
}

// pointed settles the file a position is in, and reads it.
//
// A position needs a file: a scope naming a directory names no position
// in particular, and reading the coordinate against the first file under
// it resolves something else entirely. A scope naming a file of another
// language is a scope this engine read nothing of, which is a different
// answer again and is what the second result says.
func (e *Engine) pointed(
	ctx context.Context,
	req engine.Request,
) (*session, document, bool, error) {
	if !lang.Claims(string(req.Scope), e.declared.Extensions) {
		return nil, document{}, false, nil
	}

	held, err := e.running(ctx)
	if err != nil {
		return nil, document{}, false, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if opened := e.open(ctx, held, req.Scope); opened != nil {
		return nil, document{}, false, opened
	}
	doc, err := e.read(req.Scope)
	if err != nil {
		return nil, document{}, false, err
	}
	return held, doc, true, nil
}

// definitions reads whichever arm of the answer a server chose.
//
// One location, a list of them, or a list of links. Reading only the
// list would answer nothing for every server that sends the single
// location — a name that resolves, reported as a name that does not,
// which is the failure this package is built to avoid.
func definitions(held protocol.DefinitionResult) []protocol.Location {
	switch reported := held.(type) {
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
			// The selection range, not the whole target: it names the
			// declaration's own name, which is the point a lookup has to
			// land on to find the declaration rather than whatever
			// encloses it.
			out = append(out, protocol.Location{
				URI: one.TargetURI, Range: one.TargetSelectionRange,
			})
		}
		return out
	}
	return nil
}

// finder resolves a location to the declaration written there.
//
// A protocol location is a file and a range and carries no name and no
// kind, so the file it names is outlined and the declaration covering
// the range is the answer. One call resolving forty references would
// outline the same file forty times, so what has been outlined is kept
// for the length of the call and no longer: a server's answers change as
// the workspace does.
type finder struct {
	engine  *Engine
	session *session
	held    map[source.Path]outlined
}

// outlined is one file's declarations and the text they were read from.
type outlined struct {
	symbols []sema.Symbol
	doc     document
}

func newFinder(e *Engine, held *session) *finder {
	return &finder{engine: e, session: held, held: map[source.Path]outlined{}}
}

// file outlines a path, or returns what outlining it already found.
func (f *finder) file(ctx context.Context, p source.Path) (outlined, error) {
	if held, already := f.held[p]; already {
		return held, nil
	}
	// A server answers about files outside the workspace: a standard
	// library, a module cache, a generated tree. They are not this
	// engine's to read, and a relative path climbing out of the root is
	// how one arrives.
	if !lang.Claims(string(p), f.engine.declared.Extensions) {
		f.held[p] = outlined{}
		return outlined{}, nil
	}

	// A failure here is reported rather than swallowed. The server has
	// just named this file as holding what was asked about, so being
	// unable to read it is a fault and not an empty answer, and an empty
	// answer would be indistinguishable from the file holding nothing.
	symbols, doc, err := f.engine.symbols(ctx, f.session, p)
	if err != nil {
		return outlined{}, err
	}
	held := outlined{symbols: symbols, doc: doc}
	f.held[p] = held
	return held, nil
}

// at is the declaration written at a position, innermost first.
//
// Innermost because a position inside a method is inside the type
// holding it as well, and the answer a caller wants is the one it
// pointed at rather than the one that contains it.
func (f *finder) at(
	ctx context.Context,
	p source.Path,
	at protocol.Position,
) (sema.Symbol, bool, error) {
	held, err := f.file(ctx, p)
	if err != nil {
		return sema.Symbol{}, false, err
	}
	if len(held.symbols) == 0 {
		return sema.Symbol{}, false, nil
	}

	offset := held.doc.position(at).Offset
	var found sema.Symbol
	var known bool
	for _, one := range held.symbols {
		if one.Span.Start.Offset > offset || one.Span.End.Offset < offset {
			continue
		}
		if !known || covers(found.Span) > covers(one.Span) {
			found, known = one, true
		}
	}
	return found, known, nil
}

// covers is how many bytes a span holds, which is what makes one
// declaration inside another comparable.
func covers(s source.Span) int { return s.End.Offset - s.Start.Offset }
