// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Outline reports what the files in a scope declare.
//
// One file at a time, because that is the request the protocol has: a
// server outlines a document, and a directory is the files under it. A
// scope holding many is many requests to a process that answers each in
// a millisecond once it is warm.
func (e *Engine) Outline(ctx context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	paths, err := e.files(req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	if len(paths) == 0 {
		return engine.Result[sema.Symbol]{
			Skipped: true, Completeness: trust.ScopeTotal,
		}, nil
	}

	var out []sema.Symbol
	var large []source.Path
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return engine.Result[sema.Symbol]{}, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		found, _, err := e.symbols(ctx, held, p)
		if err != nil {
			// One file past the size an engine reads costs itself and
			// not the scope. Named in a caveat, because coverage over a
			// directory holding a file nobody opened is a claim about
			// the directory rather than about its declarations.
			if _, big := errors.AsType[lang.LargeError](err); big {
				large = append(large, p)
				continue
			}
			return engine.Result[sema.Symbol]{}, err
		}
		out = append(out, found...)
	}

	covered, reaches, caveats := e.bound(ctx)
	if len(large) > 0 {
		covered = trust.ScopePartial
	}
	return engine.Result[sema.Symbol]{
		Items:        out,
		Completeness: covered,
		Lowered:      reaches,
		Caveats:      append(caveats, unread(large)...),
	}, nil
}

// dynamic is the caveat every answer from a type checker carries.
var dynamic = trust.Caveat{
	Code: trust.CaveatDynamic,
	Note: "reflection, string-keyed dispatch and struct tags are invisible here, " +
		"as they are to every engine",
}

// symbols outlines one file.
//
// The answer is a union: a tree of [protocol.DocumentSymbol], or the
// flat [protocol.SymbolInformation] a server that does not build the
// tree falls back to. Both arms are read, because a server answering the
// flat shape to a decoder expecting the tree produces a list of names
// with every field empty — an outline reporting that the file declares
// nothing, which is indistinguishable from a file that does.
// The file it read comes back with them, because every role that
// resolves a position needs the text as well as the declarations, and
// reading it once is the difference between one read per file and one
// per symbol.
func (e *Engine) symbols(
	ctx context.Context,
	held *session,
	p source.Path,
) ([]sema.Symbol, document, error) {
	if err := e.open(ctx, held, p); err != nil {
		return nil, document{}, err
	}

	if !provides(held.capable.DocumentSymbolProvider) {
		return nil, document{}, e.unsupported("textDocument/documentSymbol")
	}
	answered, err := held.asks.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
	})
	if err != nil {
		return nil, document{}, fmt.Errorf("lsp: %s: outline %s: %w", e.server.Name, p, err)
	}

	doc, err := e.read(p)
	if err != nil {
		return nil, document{}, err
	}

	unit := source.Path(e.declared.Namespace(string(p)))
	doc.names = map[int]protocol.Position{}

	var out []sema.Symbol
	switch reported := answered.(type) {
	case protocol.DocumentSymbolSlice:
		e.flatten(&out, reported, doc, unit, "")
	case protocol.SymbolInformationSlice:
		e.flat(&out, reported, doc, unit)
	}
	return out, doc, nil
}

// flatten walks the tree a server answered with, keeping the containment
// it reported rather than working it out again from spans.
func (e *Engine) flatten(
	into *[]sema.Symbol,
	held []protocol.DocumentSymbol,
	doc document,
	unit source.Path,
	parent sema.ID,
) {
	for _, one := range held {
		kind, declares := KindOf(one.Kind)
		if !declares {
			// A string or a key in a document the same request outlines.
			// It declares nothing, so it is dropped rather than reported
			// as a declaration of unknown kind.
			continue
		}

		name := trimmed(one.Name)
		id := sema.NewID(e.declared.Language, unit, name, kind)
		span := doc.span(one.Range)
		// Where the server says the name itself is written, which for a
		// documented declaration is not where the declaration starts.
		doc.names[span.Start.Offset] = one.SelectionRange.Start

		*into = append(*into, sema.Symbol{
			ID:         id,
			Name:       name,
			Kind:       kind,
			Language:   e.declared.Language,
			Span:       span,
			Parent:     parent,
			Visibility: e.declared.Visibility(name),
			Signature:  signature(name, one.SelectionRange, detail(one.Detail), doc),
			Snippet:    doc.text(span),
		})
		e.flatten(into, one.Children, doc, unit, id)
	}
}

// flat reads the shape a server answers with when it does not build the
// tree.
//
// The containment such an answer carries is a container's name, which
// cannot tell two members called Get apart, so none is reported rather
// than one that may be wrong. Everything else a caller reads — the name,
// the kind, the span, the signature — is there.
func (e *Engine) flat(
	into *[]sema.Symbol,
	held []protocol.SymbolInformation,
	doc document,
	unit source.Path,
) {
	for _, one := range held {
		kind, declares := KindOf(one.Kind)
		if !declares {
			continue
		}
		// A flat answer names the file each entry is in, and a server
		// asked about one document may answer about others it pulled in.
		if e.pathOf(one.Location.URI) != doc.path {
			continue
		}

		name := trimmed(one.Name)
		span := doc.span(one.Location.Range)

		*into = append(*into, sema.Symbol{
			ID:         sema.NewID(e.declared.Language, unit, name, kind),
			Name:       name,
			Kind:       kind,
			Language:   e.declared.Language,
			Span:       span,
			Visibility: e.declared.Visibility(name),
			Signature:  signature(name, one.Location.Range, "", doc),
			Snippet:    doc.text(span),
		})
	}
}

// signature is the declaration without its body.
//
// The line the name is on, which for every language a server serves is
// where the signature is written. Where that line is empty the server's
// own detail is used instead: gopls writes the parameters and results
// there, and a caller reading an outline wants them.
func signature(name string, at protocol.Range, detail string, doc document) string {
	held := strings.TrimSpace(doc.line(at.Start.Line))
	if held != "" {
		return held
	}
	return strings.TrimSpace(name + " " + detail)
}

// detail is what a server said about a declaration beyond its name, and
// the empty string where it said nothing.
func detail(held *string) string {
	if held == nil {
		return ""
	}
	return *held
}

// trimmed reads a name a server decorated.
//
// Servers do not agree on what a declaration is called. gopls writes a
// method as (*Store).Get, which is how it is declared rather than what
// it is called. jdtls writes it as helper(), and its call hierarchy
// writes helper() : int. A caller searches for the name, and a name
// carrying either decoration matches nothing it asks about — which is
// how a Java method came to have no callers.
//
// The qualifier goes first, then the parameters: cutting at the bracket
// first would take the whole of (*Store).Get, whose bracket opens the
// name.
func trimmed(name string) string {
	if at := strings.LastIndex(name, "."); at >= 0 {
		name = name[at+1:]
	}
	if at := strings.Index(name, "("); at > 0 {
		name = name[:at]
	}
	return strings.TrimSpace(name)
}

// files is the paths in a scope this language claims.
func (e *Engine) files(req engine.Request) ([]source.Path, error) {
	return lang.FilesIn(os.DirFS(e.root), req.Scope, e.declared.Extensions)
}
