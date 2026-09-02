// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Search reports which declarations match a query.
//
// # The answer is never total
//
// A workspace symbol query is answered from the server's own index, and
// a server caps what it returns: gopls stops at a hundred by default,
// and the cap is neither negotiated nor reported. So this reports
// [trust.ScopePartial] whatever comes back.
//
// That matters more than it looks. Resolved binding over total coverage
// is what lets a caller conclude that something does not exist, and a
// capped index cannot support that conclusion: the name it did not
// return may be the hundred and first rather than absent.
func (e *Engine) Search(
	ctx context.Context,
	req engine.Request,
	q engine.Query,
) (engine.Result[sema.Symbol], error) {
	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	if !provides(held.capable.WorkspaceSymbolProvider) {
		return engine.Result[sema.Symbol]{}, e.unsupported("workspace/symbol")
	}
	answered, err := held.asks.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: q.Text})
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("lsp: %s: search: %w", e.server.Name, err)
	}

	found := newFinder(e, held)
	var out []sema.Symbol
	for _, one := range matched(answered) {
		if err := ctx.Err(); err != nil {
			return engine.Result[sema.Symbol]{}, err
		}

		p := e.pathOf(one.uri)
		if !within(req.Scope, p) {
			continue
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}

		// The declaration is read out of the file rather than out of the
		// answer. A workspace symbol carries a name, a kind and a range
		// and no signature, no snippet and no containment, and a caller
		// searching for something wants to read it without a second call.
		declared, known, err := found.at(ctx, p, one.at.Start)
		if err != nil {
			return engine.Result[sema.Symbol]{}, err
		}
		if !known {
			declared = e.named(one, p)
		}
		if !wanted(declared, q) {
			continue
		}

		out = append(out, declared)
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}

	return engine.Result[sema.Symbol]{
		Items:        out,
		Completeness: trust.ScopePartial,
		Caveats: []trust.Caveat{dynamic, {
			Code: trust.CaveatTruncated,
			Note: "a workspace symbol query is answered from the server's own index, " +
				"which caps what it returns without saying so",
		}},
	}, nil
}

// hit is one match, in whichever shape the server sent it.
type hit struct {
	name string
	kind protocol.SymbolKind
	uri  uri.URI
	at   protocol.Range
}

// matched reads whichever arm of the answer a server chose.
//
// The newer shape may name a file and no range in it, because a server
// is allowed to defer working the range out until a caller asks for that
// one symbol. It is kept: a match with no range still names a
// declaration, and the file it is in is enough to find it.
func matched(held protocol.WorkspaceSymbolResult) []hit {
	switch reported := held.(type) {
	case protocol.SymbolInformationSlice:
		out := make([]hit, 0, len(reported))
		for _, one := range reported {
			out = append(out, hit{
				name: one.Name, kind: one.Kind,
				uri: one.Location.URI, at: one.Location.Range,
			})
		}
		return out

	case protocol.WorkspaceSymbolSlice:
		out := make([]hit, 0, len(reported))
		for _, one := range reported {
			switch where := one.Location.(type) {
			case *protocol.Location:
				out = append(out, hit{
					name: one.Name, kind: one.Kind,
					uri: where.URI, at: where.Range,
				})
			case *protocol.LocationUriOnly:
				out = append(out, hit{name: one.Name, kind: one.Kind, uri: where.URI})
			}
		}
		return out
	}
	return nil
}

// named builds a declaration from the match alone.
//
// Used where the file holding it cannot be outlined. It is worse than an
// outline — no signature, no snippet, no span — and better than dropping
// a declaration the server found.
func (e *Engine) named(one hit, p source.Path) sema.Symbol {
	kind, _ := KindOf(one.kind)
	name := trimmed(one.name)
	return sema.Symbol{
		ID: sema.NewID(e.declared.Language,
			source.Path(e.declared.Namespace(string(p))), name, kind),
		Name:       name,
		Kind:       kind,
		Language:   e.declared.Language,
		Span:       source.Span{Path: p},
		Visibility: e.declared.Visibility(name),
	}
}

// wanted applies the narrowing the query asked for and the server does
// not do.
//
// The protocol's query is one string. A kind, and whether to include
// what is not visible outside its unit, are this vocabulary's own and
// have to be applied here.
func wanted(one sema.Symbol, q engine.Query) bool {
	if q.Kind != sema.KindUnknown && one.Kind != q.Kind {
		return false
	}
	if !q.Private && one.Visibility == sema.Unexported {
		return false
	}
	return true
}

// within reports whether a path is inside a scope.
//
// A scope naming a file matches that file. A scope naming a directory
// matches what is under it, on path segments rather than on characters:
// "internal" must not claim "internalise.go".
func within(scope, p source.Path) bool {
	held := path.Clean(string(scope))
	if held == "." || held == "" {
		return true
	}
	name := path.Clean(string(p))
	return name == held || strings.HasPrefix(name, held+"/")
}
