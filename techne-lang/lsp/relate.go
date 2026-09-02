// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Relate reports how a declaration connects to the rest.
//
// # Which request answers which direction
//
// The protocol has one request per question rather than one per
// direction, and the questions do not line up with this vocabulary one
// for one:
//
//   - Referenced-by is textDocument/references, which is every use of
//     the declaration, call or not.
//   - Called-by and calls are the call hierarchy, which is narrower than
//     references and is the right answer for a caller asking about
//     calls: a name written in a type is a reference and not a call.
//   - Implements and implemented-by are both textDocument/implementation.
//     The protocol has one request and the direction follows what it is
//     pointed at: asked about an interface it names implementors, asked
//     about a type it names what that type satisfies.
//
// # What it will not answer
//
// Imports, embeds and their inverses have no request behind them. They
// are declined rather than answered with none, because none is a claim
// that there are none, and a language that has imports would be reported
// as having none of them.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !serves(kind) {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s: no request behind %s", engine.ErrDecline, e.server.Name, kind)
	}

	subject, doc, known, err := e.declaring(ctx, held, req, of)
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}
	if !known {
		// The scope holds no file this engine reads, so it says nothing
		// about the declaration rather than that it has no edges.
		return engine.Result[sema.Relation]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	at := naming(doc, subject)
	pick := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     at,
	}

	found := newFinder(e, held)
	var out []sema.Relation
	switch kind {
	case sema.ReferencedBy, sema.References:
		out, err = e.referring(ctx, held, found, pick, kind)
	case sema.CalledBy, sema.Calls:
		out, err = e.calling(ctx, held, found, pick, kind)
	case sema.Implements, sema.ImplementedBy:
		out, err = e.implementing(ctx, held, found, pick, kind)
	}
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}

	slices.SortFunc(out, order)
	return engine.Result[sema.Relation]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Caveats:      []trust.Caveat{dynamic},
	}, nil
}

// serves reports whether a direction has a request behind it.
func serves(kind sema.RelationKind) bool {
	switch kind {
	case sema.ReferencedBy, sema.References,
		sema.CalledBy, sema.Calls,
		sema.Implements, sema.ImplementedBy:
		return true
	}
	return false
}

// referring finds every use of a declaration.
//
// The declaration itself is left out. A caller asking who uses this
// already has the one it asked about, and counting it makes an unused
// declaration report one use.
func (e *Engine) referring(
	ctx context.Context,
	held *session,
	found *finder,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	sites, err := held.asks.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: pick,
		Context:                    protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: references: %w", e.server.Name, err)
	}
	return e.sited(ctx, found, sites, kind)
}

// implementing finds what a type satisfies, or what satisfies it.
func (e *Engine) implementing(
	ctx context.Context,
	held *session,
	found *finder,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	answered, err := held.asks.Implementation(ctx, &protocol.ImplementationParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: implementations: %w", e.server.Name, err)
	}
	return e.sited(ctx, found, definitions(answered), kind)
}

// sited turns locations into edges, naming the declaration each one
// falls inside.
//
// A location is a file and a range. What a caller reading who-uses-this
// wants is which declaration the use is written in, so the file is
// outlined and the declaration covering the range is the far end.
func (e *Engine) sited(
	ctx context.Context,
	found *finder,
	sites []protocol.Location,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	var out []sema.Relation
	for _, one := range sites {
		p := e.pathOf(one.URI)
		within, known, err := found.at(ctx, p, one.Range.Start)
		if err != nil {
			return nil, err
		}
		if !known {
			// A use at the top level of a file, outside every
			// declaration: an import, a package-level initialiser. It is
			// a use, and the file is what holds it.
			continue
		}
		held, err := found.file(ctx, p)
		if err != nil {
			return nil, err
		}
		span := held.doc.span(one.Range)
		out = append(out, sema.Relation{
			Kind: kind, To: within, At: span, Via: held.doc.sourceLine(span),
		})
	}
	return out, nil
}

// calling walks the call hierarchy, which is narrower than references
// and is what a caller asking about calls means.
func (e *Engine) calling(
	ctx context.Context,
	held *session,
	found *finder,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	// The hierarchy is prepared before it is walked, because the item a
	// call is reported against is the server's own handle on the
	// declaration and not a position.
	items, err := held.asks.PrepareCallHierarchy(ctx, &protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		// A server refuses this for a declaration nothing can call: an
		// interface, a type, a constant. Declining rather than failing,
		// because the question does not apply here and may apply to
		// whatever answers next — and because answering none would be a
		// claim that nothing calls it.
		return nil, fmt.Errorf("%w: %s: call hierarchy: %w", engine.ErrDecline, e.server.Name, err)
	}

	var out []sema.Relation
	for _, item := range items {
		edges, err := e.calls(ctx, held, found, item, kind)
		if err != nil {
			return nil, err
		}
		out = append(out, edges...)
	}
	return out, nil
}

// calls reads one end of the hierarchy for one item.
func (e *Engine) calls(
	ctx context.Context,
	held *session,
	found *finder,
	item protocol.CallHierarchyItem,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	var out []sema.Relation

	if kind == sema.CalledBy {
		answered, err := held.asks.IncomingCalls(ctx,
			&protocol.CallHierarchyIncomingCallsParams{Item: item})
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: incoming calls: %w", e.server.Name, err)
		}
		for _, one := range answered {
			// The ranges are relative to the caller, so the caller's own
			// file is where the call sites are written.
			edges, err := e.hierarchy(ctx, found, one.From, one.FromRanges, kind)
			if err != nil {
				return nil, err
			}
			out = append(out, edges...)
		}
		return out, nil
	}

	answered, err := held.asks.OutgoingCalls(ctx,
		&protocol.CallHierarchyOutgoingCallsParams{Item: item})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: outgoing calls: %w", e.server.Name, err)
	}
	for _, one := range answered {
		// Here the ranges are relative to the item that does the
		// calling, which is the declaration this question is about,
		// rather than to the one being called.
		edges, err := e.hierarchy(ctx, found, one.To, one.FromRanges, kind)
		if err != nil {
			return nil, err
		}
		for i := range edges {
			edges[i].At = source.Span{}
			edges[i].Via = ""
		}
		out = append(out, edges...)
	}
	return out, nil
}

// hierarchy turns one call-hierarchy item and its sites into edges.
//
// The item carries the name and the kind the server assigned, so the far
// end is built from it rather than from outlining the file again — which
// for a caller in a file this language does not claim is the only way to
// name it at all.
func (e *Engine) hierarchy(
	ctx context.Context,
	found *finder,
	item protocol.CallHierarchyItem,
	sites []protocol.Range,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	p := e.pathOf(item.URI)
	held, err := found.file(ctx, p)
	if err != nil {
		return nil, err
	}

	far, known, err := found.at(ctx, p, item.SelectionRange.Start)
	if err != nil {
		return nil, err
	}
	if !known {
		far = e.itemised(item, p)
	}

	if len(sites) == 0 {
		return []sema.Relation{{Kind: kind, To: far}}, nil
	}

	out := make([]sema.Relation, 0, len(sites))
	for _, at := range sites {
		span := source.Span{Path: p}
		via := ""
		if len(held.doc.at) > 0 {
			span = held.doc.span(at)
			via = held.doc.sourceLine(span)
		}
		out = append(out, sema.Relation{Kind: kind, To: far, At: span, Via: via})
	}
	return out, nil
}

// itemised builds a declaration from what the hierarchy said about it.
//
// Used where the file cannot be outlined: a caller in a dependency, in a
// generated tree, in a language this engine does not claim. Naming it
// from the item is worse than an outline and better than dropping an
// edge that exists.
func (e *Engine) itemised(item protocol.CallHierarchyItem, p source.Path) sema.Symbol {
	kind, _ := KindOf(item.Kind)
	name := trimmed(item.Name)
	return sema.Symbol{
		ID:         sema.NewID(e.declared.Language, source.Path(e.declared.Namespace(string(p))), name, kind),
		Name:       name,
		Kind:       kind,
		Language:   e.declared.Language,
		Span:       source.Span{Path: p},
		Visibility: e.declared.Visibility(name),
		Signature:  strings.TrimSpace(name + " " + detail(item.Detail)),
	}
}

// declaring finds the declaration a question is about, and the file it
// is written in.
func (e *Engine) declaring(
	ctx context.Context,
	held *session,
	req engine.Request,
	of sema.ID,
) (sema.Symbol, document, bool, error) {
	paths, err := e.files(req)
	if err != nil {
		return sema.Symbol{}, document{}, false, err
	}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return sema.Symbol{}, document{}, false, err
		}
		symbols, doc, err := e.symbols(ctx, held, p)
		if err != nil {
			return sema.Symbol{}, document{}, false, err
		}
		for _, one := range symbols {
			if one.ID == of {
				return one, doc, true, nil
			}
		}
	}
	return sema.Symbol{}, document{}, false, nil
}

// naming is where a declaration's own name is written.
//
// A request about a declaration has to point at its name. A server asked
// about the first byte of "type Store struct {" is asked about the
// keyword and answers about nothing, so the name is found inside the
// declaration's own text rather than assumed to be at its start.
func naming(doc document, of sema.Symbol) protocol.Position {
	if at := worded(doc.text(of.Span), of.Name); at >= 0 {
		return doc.mark(source.Position{Offset: of.Span.Start.Offset + at})
	}
	return doc.mark(of.Span.Start)
}

// worded finds a name written as a name, and not as part of a longer
// one.
//
// Without the boundary check, resolving Get inside a declaration holding
// GetAll points the request at the wrong four bytes and the server
// answers about something else.
func worded(within, name string) int {
	if name == "" {
		return -1
	}
	for from := 0; ; {
		at := strings.Index(within[from:], name)
		if at < 0 {
			return -1
		}
		at += from
		if !joined(within, at-1) && !joined(within, at+len(name)) {
			return at
		}
		from = at + len(name)
	}
}

// joined reports whether the byte at an index could be part of a name.
func joined(within string, at int) bool {
	if at < 0 || at >= len(within) {
		return false
	}
	r := rune(within[at])
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// order sorts edges by where they were written, so an answer reads in
// file order and does not wander between calls.
func order(a, b sema.Relation) int {
	if by := strings.Compare(string(a.At.Path), string(b.At.Path)); by != 0 {
		return by
	}
	if by := a.At.Start.Offset - b.At.Start.Offset; by != 0 {
		return by
	}
	return strings.Compare(a.To.Name, b.To.Name)
}
