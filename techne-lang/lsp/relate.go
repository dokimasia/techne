// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Relate returns the relations of one kind from a declaration, sorted by the path and offset
// of their sites.
//
// Each kind has one request of LSP 3.17:
//
//   - [sema.ReferencedBy] and [sema.References]: textDocument/references, every use of the
//     declaration, the declaration itself excluded.
//   - [sema.CalledBy] and [sema.Calls]: the incoming and outgoing calls of the call hierarchy.
//     A use in a type is a reference and not a call.
//   - [sema.Implements] and [sema.ImplementedBy]: textDocument/implementation, whose direction
//     depends on the declaration it is asked about.
//   - [sema.Embeds] and [sema.EmbeddedBy]: the supertypes and subtypes of the type hierarchy.
//
// Imports and their inverse have no request. Relate returns [engine.ErrDecline] for them, so
// the tree-sitter engine reads them from the source.
//
// Relate finds the declaration in the file of [engine.Request.Declared] when the request names
// one, and asks the server about the name in that span when no symbol of the server matches
// the ID. Without a declared span it searches the files of the scope, and a declaration that no
// file of the scope declares returns [engine.ErrDecline]. For a scope without a file of the
// language the result is skipped. A server that does not answer within [Server.Answering]
// returns [engine.ErrDecline].
//
// Relate reads the declaration that contains each of the first [engine.Request.Limit] sites
// that the server names, by path and position, and returns their relations. The outline engine
// of the language reads those declarations when the engine has one, so the server opens only
// the file of the declaration that the question is about. A site at the name of another
// declaration with the ID is left out, such as the definition of a C function whose prototype
// the question is about. A caveat of [trust.CaveatTruncated] counts the sites that the answer
// leaves out. An error lowers the answer when it is on a line that writes the name of the
// declaration outside every site that the server named.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	out, err := e.relating(ctx, req, of, kind)
	return out, e.unanswered(ctx, err)
}

// relating is [Engine.Relate] before a missed deadline becomes a decline.
func (e *Engine) relating(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	files, err := e.walk(req)
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}
	if len(files.Read) == 0 && len(files.Unread) == 0 {
		return engine.Result[sema.Relation]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}
	if !related(kind) {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s: no request returns %s", engine.ErrDecline, e.server.Name, kind)
	}

	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	ctx, done := e.answered(ctx)
	defer done()
	found := newFinder(e, held)
	subject, doc, known, err := e.declaring(ctx, found, req, of, files.Read)
	switch {
	case err != nil:
		return engine.Result[sema.Relation]{}, err
	case !known:
		return engine.Result[sema.Relation]{}, fmt.Errorf("%w: %s: no declaration in %s matches %s%s",
			engine.ErrDecline, e.server.Name, req.Scope, of, skipping(files.Unread))
	}
	// The files are open, and opening a file starts its analysis in the server.
	ready := e.settle(ctx, held)

	pick := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
		Position:     naming(doc, subject),
	}
	var named []site
	// saw reports whether the server returned any handle on the declaration. An empty reply
	// from a server without one is no evidence that the declaration has no relation.
	saw := true
	switch kind {
	case sema.ReferencedBy, sema.References:
		named, saw, err = e.referring(ctx, held, pick)
	case sema.CalledBy, sema.Calls:
		named, saw, err = e.calling(ctx, held, pick, kind)
	case sema.Implements, sema.ImplementedBy:
		named, saw, err = e.implementing(ctx, held, pick)
	case sema.Embeds, sema.EmbeddedBy:
		named, saw, err = e.incorporating(ctx, held, pick, kind)
	}
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}

	kept, left := limited(named, req.Limit)
	out := make([]sema.Relation, 0, len(kept))
	for _, one := range kept {
		edge, declares, err := e.relation(ctx, found, kind, one, of)
		if err != nil {
			return engine.Result[sema.Relation]{}, err
		}
		if !declares {
			out = append(out, edge)
		}
	}
	slices.SortFunc(out, order)

	sites := []source.Span{subject.Span}
	for _, one := range out {
		sites = append(sites, one.At)
	}
	for _, one := range left {
		sites = append(sites, one.lines())
	}
	covered, reaches, caveats := e.bound(held, req.Scope, ready, lang.Writing(subject.Name, lang.Spanned(sites...)))
	if !saw {
		covered = trust.ScopePartial
		caveats = append(caveats, unresolved)
	}
	if len(left) > 0 {
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatTruncated,
			Note: fmt.Sprintf("%d of %d relations returned", len(out), len(named)),
		})
	}
	return engine.Result[sema.Relation]{
		Items:        out,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      caveats,
	}, nil
}

// related reports whether a request of LSP 3.17 returns the relations of kind.
func related(kind sema.RelationKind) bool {
	switch kind {
	case sema.ReferencedBy, sema.References,
		sema.CalledBy, sema.Calls,
		sema.Implements, sema.ImplementedBy,
		sema.Embeds, sema.EmbeddedBy:
		return true
	}
	return false
}

// skipping returns the clause of a decline that counts the files larger than [lang.Largest]
// that the search for a declaration skipped, or the empty string for none.
func skipping(unread []source.Path) string {
	if len(unread) == 0 {
		return ""
	}
	return fmt.Sprintf(", and %d files larger than an engine reads were not searched", len(unread))
}

// site is one relation that a server named, before the engine reads the declaration at its
// far end.
type site struct {
	// path is the file of at, the range of the call or the reference.
	path source.Path
	at   protocol.Range
	// placed reports whether the server returned a range for the site. The call hierarchy can
	// return a far end without one.
	placed bool
	// far is the item that the server named as the far end, or nil when the far end is the
	// innermost declaration that contains the site.
	far *protocol.CallHierarchyItem
}

// lines returns the span of the lines of s, without offsets.
func (s site) lines() source.Span {
	return source.Span{
		Path:  s.path,
		Start: source.Position{Line: int(s.at.Start.Line)},
		End:   source.Position{Line: int(s.at.End.Line)},
	}
}

// limited sorts sites by path and position, and returns the first limit of them and the sites
// after those. A limit of zero or less keeps every site.
func limited(sites []site, limit int) (kept, left []site) {
	slices.SortStableFunc(sites, func(a, b site) int {
		return cmp.Or(
			cmp.Compare(a.path, b.path),
			cmp.Compare(a.at.Start.Line, b.at.Start.Line),
			cmp.Compare(a.at.Start.Character, b.at.Start.Character),
		)
	})
	if limit <= 0 || len(sites) <= limit {
		return sites, nil
	}
	return sites[:limit], sites[limit:]
}

// relation returns the relation of kind that s names, and reports whether s is a declaration
// of subject in place of a use.
//
// The far end of a site with an item is the declaration at the selection range of the item,
// or a declaration built from the item when its file has none there. The far end of a site
// without an item is the innermost declaration that contains the site and that its file offers
// to the rest of a program, or the file of the site when no such declaration contains it, as
// for an import. The finder reads each file of a site without the server when the engine has
// an outline engine.
//
// A reference at the name of a declaration with the ID subject is a declaration: a server
// reports the definition of a C function among the references of its prototype.
func (e *Engine) relation(
	ctx context.Context,
	found *finder,
	kind sema.RelationKind,
	s site,
	subject sema.ID,
) (sema.Relation, bool, error) {
	var to sema.Symbol
	if s.far != nil {
		farPath := e.pathOf(s.far.URI)
		declared, known, err := found.at(ctx, farPath, s.far.SelectionRange.Start)
		if err != nil {
			return sema.Relation{}, false, err
		}
		to = declared
		if !known {
			to = e.itemised(*s.far, farPath)
		}
		if !s.placed {
			return sema.Relation{Kind: kind, To: to}, false, nil
		}
	}

	kept, err := found.file(ctx, s.path)
	if err != nil {
		return sema.Relation{}, false, err
	}
	at, via := siteOf(kept.doc, s.path, s.at)
	declares := false
	if s.far == nil {
		inside, known, err := found.within(ctx, s.path, s.at.Start)
		if err != nil {
			return sema.Relation{}, false, err
		}
		to = inside
		if !known {
			to = e.fileOf(kept.doc, s.path, at)
		}
		declares = referencing(kind) && known && inside.ID == subject && named(kept.doc, inside) == at.Start.Offset
	}
	return sema.Relation{Kind: kind, To: to, At: at, Via: via}, declares, nil
}

// referencing reports whether kind is a direction of textDocument/references.
func referencing(kind sema.RelationKind) bool {
	return kind == sema.ReferencedBy || kind == sema.References
}

// named returns the offset in doc of the first occurrence of the name of d as a word in the
// source of d, or -1 when the source does not contain it.
func named(doc document, d sema.Symbol) int {
	at := lang.Worded(doc.text(d.Span), d.Name)
	if at < 0 {
		return -1
	}
	return d.Span.Start.Offset + at
}

// located returns the site of a location whose far end is the declaration that contains it.
func (e *Engine) located(one protocol.Location) site {
	return site{path: e.pathOf(one.URI), at: one.Range, placed: true}
}

// referring returns the sites of the uses of the declaration at pick. It asks with the
// declaration included and drops the declaration, so an empty reply shows that the server has
// no handle on the declaration, which the second result reports.
func (e *Engine) referring(
	ctx context.Context,
	held *session,
	pick protocol.TextDocumentPositionParams,
) ([]site, bool, error) {
	if !provides(held.capable.ReferencesProvider) {
		return nil, false, e.unsupported("textDocument/references")
	}
	answered, err := held.asks.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: pick,
		Context:                    protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		return nil, false, fmt.Errorf("lsp: %s: references: %w", e.server.Name, err)
	}
	if len(answered) == 0 {
		return nil, false, nil
	}
	out := make([]site, 0, len(answered))
	for _, one := range answered {
		if !pointsAt(one, pick) {
			out = append(out, e.located(one))
		}
	}
	return out, true, nil
}

// pointsAt reports whether the range of one contains the position of pick in the same file.
func pointsAt(one protocol.Location, pick protocol.TextDocumentPositionParams) bool {
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

// implementing returns the sites of what the declaration at pick implements, or of what
// implements it, from textDocument/implementation. An empty reply is evidence of none only
// from a server that has analysed the file, which the second result reports.
func (e *Engine) implementing(
	ctx context.Context,
	held *session,
	pick protocol.TextDocumentPositionParams,
) ([]site, bool, error) {
	if !provides(held.capable.ImplementationProvider) {
		return nil, false, e.unsupported("textDocument/implementation")
	}
	answered, err := held.asks.Implementation(ctx, &protocol.ImplementationParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		return nil, false, fmt.Errorf("lsp: %s: implementations: %w", e.server.Name, err)
	}
	locations := definitions(answered)
	if len(locations) == 0 {
		return nil, e.analysed(ctx, held, e.pathOf(pick.TextDocument.URI)), nil
	}
	out := make([]site, 0, len(locations))
	for _, one := range locations {
		out = append(out, e.located(one))
	}
	return out, true, nil
}

// siteOf returns the span of r in doc and the source line it starts on. For a file without a
// document, such as a file larger than [lang.Largest], it returns a span with the lines that
// the server reported, no offsets, and an empty line.
func siteOf(doc document, p source.Path, r protocol.Range) (source.Span, string) {
	if len(doc.at) == 0 {
		return source.Span{
			Path:  p,
			Start: source.Position{Line: int(r.Start.Line)},
			End:   source.Position{Line: int(r.End.Line)},
		}, ""
	}
	span := doc.span(r)
	return span, doc.sourceLine(span)
}

// fileOf returns the file at p as the far end of a relation whose site no declaration
// contains. Its kind is [sema.KindFile], which [sema.Kinds] lists as a kind an answer
// may contain.
func (e *Engine) fileOf(doc document, p source.Path, at source.Span) sema.Symbol {
	return sema.Symbol{
		Name:     path.Base(string(p)),
		Kind:     sema.KindFile,
		Language: e.declared.Language,
		Span:     at,
		Snippet:  doc.text(at),
	}
}

// calling returns the sites of the incoming or the outgoing calls of the declaration at pick,
// from the call hierarchy. A server that refuses to prepare the hierarchy, as servers do for a
// declaration that cannot be called, returns [engine.ErrDecline]. A server that prepares no
// item has no handle on the declaration, which the second result reports.
func (e *Engine) calling(
	ctx context.Context,
	held *session,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]site, bool, error) {
	if !provides(held.capable.CallHierarchyProvider) {
		return nil, false, e.unsupported("the call hierarchy")
	}
	items, err := held.asks.PrepareCallHierarchy(ctx, &protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		return nil, false, fmt.Errorf("%w: %s: call hierarchy: %w", engine.ErrDecline, e.server.Name, err)
	}
	if len(items) == 0 {
		return nil, false, nil
	}

	var out []site
	for _, item := range items {
		sites, err := e.calls(ctx, held, item, kind)
		if err != nil {
			return nil, false, err
		}
		out = append(out, sites...)
	}
	return out, true, nil
}

// calls returns the sites of the calls of one call hierarchy item. The sites of a call are
// ranges in the file of the caller: the caller of an incoming call, and the item itself for an
// outgoing call.
func (e *Engine) calls(
	ctx context.Context,
	held *session,
	item protocol.CallHierarchyItem,
	kind sema.RelationKind,
) ([]site, error) {
	var out []site
	if kind == sema.CalledBy {
		answered, err := held.asks.IncomingCalls(ctx, &protocol.CallHierarchyIncomingCallsParams{Item: item})
		if err != nil {
			return nil, fmt.Errorf("lsp: %s: incoming calls: %w", e.server.Name, err)
		}
		for _, one := range answered {
			out = append(out, e.hierarchy(one.From, one.From.URI, one.FromRanges)...)
		}
		return out, nil
	}

	answered, err := held.asks.OutgoingCalls(ctx, &protocol.CallHierarchyOutgoingCallsParams{Item: item})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: outgoing calls: %w", e.server.Name, err)
	}
	for _, one := range answered {
		out = append(out, e.hierarchy(one.To, item.URI, one.FromRanges)...)
	}
	return out, nil
}

// hierarchy returns one site per range of ranges, each a range in the file of in, with far as
// the far end. A far end without ranges has one site without a range.
func (e *Engine) hierarchy(far protocol.CallHierarchyItem, in uri.URI, ranges []protocol.Range) []site {
	if len(ranges) == 0 {
		return []site{{far: &far}}
	}
	p := e.pathOf(in)
	out := make([]site, 0, len(ranges))
	for _, r := range ranges {
		out = append(out, site{path: p, at: r, placed: true, far: &far})
	}
	return out
}

// itemised returns the declaration that a hierarchy item describes, for a file whose
// declarations the engine has not read. It has a name, a kind and a signature, and its span
// names the file only.
func (e *Engine) itemised(item protocol.CallHierarchyItem, p source.Path) sema.Symbol {
	kind, _ := KindOf(item.Kind)
	name := trimmed(item.Name)
	unit := source.Path(e.declared.Namespace(string(p)))
	return sema.Symbol{
		ID:         sema.NewID(e.declared.Language, unit, qualified(kind, name, "", item.Name), kind),
		Name:       name,
		Kind:       kind,
		Language:   e.declared.Language,
		Span:       source.Span{Path: p},
		Visibility: e.declared.Visibility(name),
		Signature:  strings.TrimSpace(name + " " + optional(item.Detail)),
	}
}

// incorporating returns the sites of the supertypes or the subtypes of the type at pick, from
// the type hierarchy. A server that refuses to prepare the hierarchy, as servers do for a
// declaration that is not a type, returns [engine.ErrDecline]. A server that prepares no item
// has no handle on the type, which the second result reports.
func (e *Engine) incorporating(
	ctx context.Context,
	held *session,
	pick protocol.TextDocumentPositionParams,
	kind sema.RelationKind,
) ([]site, bool, error) {
	if !provides(held.capable.TypeHierarchyProvider) {
		return nil, false, e.unsupported("the type hierarchy")
	}
	items, err := held.asks.PrepareTypeHierarchy(ctx, &protocol.TypeHierarchyPrepareParams{
		TextDocumentPositionParams: pick,
	})
	if err != nil {
		return nil, false, fmt.Errorf("%w: %s: type hierarchy: %w", engine.ErrDecline, e.server.Name, err)
	}
	if len(items) == 0 {
		return nil, false, nil
	}

	var out []site
	for _, item := range items {
		related, err := e.typed(ctx, held, item, kind)
		if err != nil {
			return nil, false, err
		}
		for _, one := range related {
			called := protocol.CallHierarchyItem{
				Name: one.Name, Kind: one.Kind, Detail: one.Detail,
				URI: one.URI, Range: one.Range, SelectionRange: one.SelectionRange,
			}
			out = append(out, e.hierarchy(called, one.URI, []protocol.Range{one.SelectionRange})...)
		}
	}
	return out, true, nil
}

// typed returns the supertypes of item for [sema.Embeds] and its subtypes otherwise.
func (e *Engine) typed(
	ctx context.Context,
	held *session,
	item protocol.TypeHierarchyItem,
	kind sema.RelationKind,
) ([]protocol.TypeHierarchyItem, error) {
	if kind == sema.Embeds {
		out, err := held.asks.Supertypes(ctx, &protocol.TypeHierarchySupertypesParams{Item: item})
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

// order compares two relations by the path of their site, then its offset, then the name of
// the far end.
func order(a, b sema.Relation) int {
	return cmp.Or(
		cmp.Compare(a.At.Path, b.At.Path),
		cmp.Compare(a.At.Start.Offset, b.At.Start.Offset),
		cmp.Compare(a.To.Name, b.To.Name),
	)
}
