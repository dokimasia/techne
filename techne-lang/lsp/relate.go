// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"path"
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
//   - Embeds and embedded-by are the type hierarchy, which is what a
//     type incorporates and what incorporates it. The languages spell it
//     differently — an anonymous field, extends, with, include — and the
//     hierarchy is the one request that answers all of them.
//
// # What it will not answer
//
// Imports and their inverse, which are written in the source rather than
// resolved from it and are the parser's to read. Declined rather than
// answered with none, because none is a claim that there are none.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	// Read before the server is started, and the same paths are what
	// declaring works through: the scope is walked once either way, and
	// a scope holding none of this language never costs a process.
	paths, err := e.files(req)
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}
	if len(paths) == 0 {
		// The scope holds no file this engine reads, so it says nothing
		// about the declaration rather than that it has no edges.
		return engine.Result[sema.Relation]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	if !serves(kind) {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s: no request behind %s", engine.ErrDecline, e.server.Name, kind)
	}

	subject, doc, known, read, err := e.declaring(ctx, held, req, of, paths)
	switch {
	case err != nil:
		return engine.Result[sema.Relation]{}, err
	case !read:
		// The scope holds no file this engine reads, so it says nothing
		// about the declaration rather than that it has no edges.
		return engine.Result[sema.Relation]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	case !known:
		// Read the files and found no such declaration. Declined rather
		// than answered with none: an engine that cannot find what it
		// was asked about has no view of its edges either, and none of
		// them over total coverage is a claim that it has none.
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s: no declaration in %q matches %s",
			engine.ErrDecline, e.server.Name, req.Scope, of)
	}

	// Waited on here rather than before the files were opened. Opening a
	// document is what starts a server compiling it, so a wait that ran
	// first waited on an idle server and the question that followed
	// raced the work it had just asked for.
	e.working.settle(ctx, e.settling())

	at := naming(doc, subject)
	pick := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     at,
	}

	found := newFinder(e, held)
	var out []sema.Relation
	// saw reports whether the server resolved the declaration at all.
	// An empty answer from a server that did not is not a claim that
	// there are no edges; it is a server with nothing loaded to answer
	// from, and the two are indistinguishable in the answer itself.
	saw := true
	switch kind {
	case sema.ReferencedBy, sema.References:
		out, saw, err = e.referring(ctx, held, found, pick, kind)
	case sema.CalledBy, sema.Calls:
		out, saw, err = e.calling(ctx, held, found, pick, kind)
	case sema.Implements, sema.ImplementedBy:
		out, saw, err = e.implementing(ctx, held, found, pick, kind)
	case sema.Embeds, sema.EmbeddedBy:
		out, saw, err = e.incorporating(ctx, held, found, pick, kind)
	}
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}

	slices.SortFunc(out, order)
	covered, reaches, caveats := e.bound(ctx, req.Scope)
	if !saw {
		covered = trust.ScopePartial
		caveats = append(caveats, unresolved)
	}
	return engine.Result[sema.Relation]{
		Items:        out,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// unresolved is the caveat on an empty answer nothing stands behind.
//
// What it says and does not say matters. It does not say the server is
// broken or that the declaration is unknown to it: metals answers
// references for a trait whose implementations it reports as none. It
// says only that nothing established the server had analysed the file,
// so an empty answer may be one it has not looked at rather than one
// with nothing in it — and those are the two an empty list cannot tell
// apart.
//
// The caution is deliberate and it costs something: a server that
// analysed a file, found nothing, and published nothing to say so is
// reported as short when it was complete. That is the wrong way round to
// be wrong, which is the only reason to prefer it.
var unresolved = trust.Caveat{
	Code: trust.CaveatIndexWarming,
	Note: "nothing established that the server had analysed this file, so an " +
		"empty answer may be one it has not looked at",
}

// serves reports whether a direction has a request behind it.
func serves(kind sema.RelationKind) bool {
	switch kind {
	case sema.ReferencedBy, sema.References,
		sema.CalledBy, sema.Calls,
		sema.Implements, sema.ImplementedBy,
		sema.Embeds, sema.EmbeddedBy:
		return true
	}
	return false
}

// incorporating walks the type hierarchy, which is what one type takes
// from another.
//
// Every language spells it differently and the protocol has one request
// for all of them: an anonymous field in Go, extends in Java and
// TypeScript, with in Scala, include in Ruby, a base class in Python.
// Supertypes are what a type incorporates and subtypes are what
// incorporates it.
func (e *Engine) incorporating(
	ctx context.Context,
	held *session,
	found *finder,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]sema.Relation, bool, error) {
	if !provides(held.capable.TypeHierarchyProvider) {
		return nil, false, e.unsupported("the type hierarchy")
	}
	items, err := held.asks.PrepareTypeHierarchy(ctx, &protocol.TypeHierarchyPrepareParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		// A server refuses this for anything that is not a type, which
		// is a question that does not apply rather than one with no
		// answer.
		return nil, false, fmt.Errorf("%w: %s: type hierarchy: %w",
			engine.ErrDecline, e.server.Name, err)
	}
	// No item is the server saying it has no handle on this type, which
	// is a different fact from the type incorporating nothing.
	if len(items) == 0 {
		return nil, false, nil
	}

	var out []sema.Relation
	for _, item := range items {
		related, err := e.hierarchical(ctx, held, item, kind)
		if err != nil {
			return nil, false, err
		}
		for _, one := range related {
			edges, err := e.typed(ctx, found, one, kind)
			if err != nil {
				return nil, false, err
			}
			out = append(out, edges...)
		}
	}
	return out, true, nil
}

// hierarchical reads one end of the hierarchy for one item.
func (e *Engine) hierarchical(
	ctx context.Context,
	held *session,
	item protocol.TypeHierarchyItem,
	kind sema.RelationKind,
) ([]protocol.TypeHierarchyItem, error) {
	if kind == sema.Embeds {
		out, err := held.asks.Supertypes(ctx,
			&protocol.TypeHierarchySupertypesParams{Item: item})
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: supertypes: %w", e.server.Name, err)
		}
		return out, nil
	}
	out, err := held.asks.Subtypes(ctx, &protocol.TypeHierarchySubtypesParams{Item: item})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: subtypes: %w", e.server.Name, err)
	}
	return out, nil
}

// typed turns one type-hierarchy item into an edge, naming it from an
// outline where the file can be read and from the item where it cannot.
func (e *Engine) typed(
	ctx context.Context,
	found *finder,
	item protocol.TypeHierarchyItem,
	kind sema.RelationKind,
) ([]sema.Relation, error) {
	p := e.pathOf(item.URI)
	far, known, err := found.at(ctx, p, item.SelectionRange.Start)
	if err != nil {
		return nil, err
	}
	if !known {
		far = e.itemised(hierarchyItem(item), p)
	}

	doc, err := found.file(ctx, p)
	if err != nil {
		return nil, err
	}
	span := source.Span{Path: p}
	via := ""
	if len(doc.doc.at) > 0 {
		span = doc.doc.span(item.SelectionRange)
		via = doc.doc.sourceLine(span)
	}
	return []sema.Relation{{Kind: kind, To: far, At: span, Via: via}}, nil
}

// hierarchyItem reads a type-hierarchy item as a call-hierarchy one, so
// one conversion names both. The two carry the same fields under the
// same names and differ only in which request produced them.
func hierarchyItem(held protocol.TypeHierarchyItem) protocol.CallHierarchyItem {
	return protocol.CallHierarchyItem{
		Name: held.Name, Kind: held.Kind, Detail: held.Detail,
		URI: held.URI, Range: held.Range, SelectionRange: held.SelectionRange,
	}
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
) ([]sema.Relation, bool, error) {
	if !provides(held.capable.ReferencesProvider) {
		return nil, false, e.unsupported("textDocument/references")
	}
	// Asked with the declaration included, and it is dropped here. A
	// server that resolved the declaration returns at least the
	// declaration; one that returns nothing at all did not resolve it,
	// and its silence says nothing about how many uses there are.
	sites, err := held.asks.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: pick,
		Context:                    protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		return nil, false, fmt.Errorf("lsp: %s: references: %w", e.server.Name, err)
	}
	if len(sites) == 0 {
		return nil, false, nil
	}

	uses := make([]protocol.Location, 0, len(sites))
	for _, one := range sites {
		if declaring(one, pick) {
			continue
		}
		uses = append(uses, one)
	}
	edges, err := e.sited(ctx, found, uses, kind)
	return edges, true, err
}

// declaring reports whether a location is the declaration the question
// was about, rather than a use of it.
func declaring(one protocol.Location, pick protocol.TextDocumentPositionParams) bool {
	if one.URI != pick.TextDocument.URI {
		return false
	}
	at, from, to := pick.Position, one.Range.Start, one.Range.End
	if at.Line < from.Line || at.Line > to.Line {
		return false
	}
	if at.Line == from.Line && at.Character < from.Character {
		return false
	}
	return at.Line != to.Line || at.Character <= to.Character
}

// implementing finds what a type satisfies, or what satisfies it.
func (e *Engine) implementing(
	ctx context.Context,
	held *session,
	found *finder,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]sema.Relation, bool, error) {
	if !provides(held.capable.ImplementationProvider) {
		return nil, false, e.unsupported("textDocument/implementation")
	}
	answered, err := held.asks.Implementation(ctx, &protocol.ImplementationParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		return nil, false, fmt.Errorf("lsp: %s: implementations: %w", e.server.Name, err)
	}

	sites := definitions(answered)
	edges, err := e.sited(ctx, found, sites, kind)
	if err != nil || len(sites) > 0 {
		return edges, true, err
	}
	// Nothing came back, and for a concrete type that is the right
	// answer. Whether it is depends on the server having a view of the
	// file at all, which the empty list cannot say and which resolving a
	// definition does not establish: a syntactic index resolves one.
	return edges, e.analysed(ctx, held, e.pathOf(pick.TextDocument.URI)), nil
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
		held, err := found.file(ctx, p)
		if err != nil {
			return nil, err
		}
		if !known {
			// The site falls outside every declaration the outline
			// reports: an import, a package-level initialiser, an impl
			// block a server does not report as a symbol. It is still a
			// site, so it is named by the file that holds it rather than
			// dropped — an edge that exists and is not reported is the
			// same false answer as one that was never found.
			within = at(e, held, one.Range)
		}
		span := held.doc.span(one.Range)
		out = append(out, sema.Relation{
			Kind: kind, To: within, At: span, Via: held.doc.sourceLine(span),
		})
	}
	return out, nil
}

// at names a site by the file that holds it, for a site no declaration
// encloses.
//
// Worse than an outline and better than silence: it carries the file,
// the line and the source, which is what a caller reading a list of
// sites acts on. An import, a package-level initialiser and an impl
// block a server does not report as a symbol all land here.
//
// The file, rather than the kind for a declaration nobody classified.
// [sema.Kinds] leaves that one out of the set an answer may carry, so an
// item holding it does not validate against the shape this tool
// declares — a fuzzing run over a real workspace found exactly that,
// and a client checking the schema would drop the whole answer.
func at(e *Engine, held outlined, over protocol.Range) sema.Symbol {
	span := held.doc.span(over)
	return sema.Symbol{
		Name:     path.Base(string(held.doc.path)),
		Kind:     sema.KindFile,
		Language: e.declared.Language,
		Span:     span,
		Snippet:  held.doc.text(span),
	}
}

// calling walks the call hierarchy, which is narrower than references
// and is what a caller asking about calls means.
func (e *Engine) calling(
	ctx context.Context,
	held *session,
	found *finder,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]sema.Relation, bool, error) {
	// The hierarchy is prepared before it is walked, because the item a
	// call is reported against is the server's own handle on the
	// declaration and not a position.
	if !provides(held.capable.CallHierarchyProvider) {
		return nil, false, e.unsupported("the call hierarchy")
	}
	items, err := held.asks.PrepareCallHierarchy(ctx, &protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		// A server refuses this for a declaration nothing can call: an
		// interface, a type, a constant. Declining rather than failing,
		// because the question does not apply here and may apply to
		// whatever answers next — and because answering none would be a
		// claim that nothing calls it.
		return nil, false, fmt.Errorf("%w: %s: call hierarchy: %w",
			engine.ErrDecline, e.server.Name, err)
	}
	// No item is the server saying it has no handle on this declaration,
	// which is a different fact from the declaration having no calls.
	if len(items) == 0 {
		return nil, false, nil
	}

	var out []sema.Relation
	for _, item := range items {
		edges, err := e.calls(ctx, held, found, item, kind)
		if err != nil {
			return nil, false, err
		}
		out = append(out, edges...)
	}
	return out, true, nil
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
// The last result reports whether there was anything to read at all,
// which is a different answer from having read it and found no such
// declaration: the first says nothing about the language, and the second
// says this engine cannot answer about this name.
func (e *Engine) declaring(
	ctx context.Context,
	held *session,
	req engine.Request,
	of sema.ID,
	paths []source.Path,
) (found sema.Symbol, doc document, known, read bool, err error) {
	// Collected rather than returned on the first match, because the
	// fallback below has to know whether a name picks out one
	// declaration or several.
	var named []sema.Symbol
	var where []document
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return sema.Symbol{}, document{}, false, false, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		read = true

		symbols, doc, err := e.symbols(ctx, held, p)
		if err != nil {
			return sema.Symbol{}, document{}, false, read, err
		}
		for _, one := range symbols {
			if one.ID == of {
				return one, doc, true, read, nil
			}
			if one.Name == of.Name() {
				named = append(named, one)
				where = append(where, doc)
			}
		}
	}

	// No identity matched. An identity carries a kind, and the parser
	// that built the one being asked about and the server answering here
	// need not agree on it: metals calls a method in a Scala object what
	// the parser calls a function, and the two identities differ in that
	// one field alone. Falling back to the name settles it wherever the
	// name picks out one declaration, and refuses to guess where it does
	// not.
	if len(named) == 1 {
		return named[0], where[0], true, read, nil
	}
	return sema.Symbol{}, document{}, false, read, nil
}

// naming is where a declaration's own name is written.
//
// A request about a declaration has to point at its name. A server asked
// about the first byte of "type Store struct {" is asked about the
// keyword and answers about nothing.
//
// The server says where the name is, and that is taken: a declaration's
// span starts at its documentation for the servers that count the
// documentation as part of it, and a declaration documented with its own
// name in the first line — which is what a doc comment is — would
// otherwise have every request about it aimed at the comment. Driving a
// rename over a documented Rust struct produced exactly that, and
// rust-analyzer answered that there is nothing there.
//
// Searching the text is the fallback, for a server that answers the flat
// shape and says only where the whole declaration is.
func naming(doc document, of sema.Symbol) protocol.Position {
	if at, said := doc.names[of.Span.Start.Offset]; said {
		return at
	}
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
