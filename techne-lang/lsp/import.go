// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// byName is the caveat of every answer of [Engine.importers], which comes with
// [trust.ScopePartial].
var byName = trust.Caveat{
	Code: trust.CaveatUnsupported,
	Note: "the parser finds an import by the name that it writes, and the server resolved each import " +
		"that it found: an import under another name, such as an import of a directory, is not read",
}

// reasking is the pause before [Engine.importers] asks again at the imports that a starting
// server resolved to no file.
const reasking = time.Second

// resolution is what the definitions at the names of one import show about the target of
// [Engine.importers]. A stronger resolution has a greater value.
type resolution uint8

// The resolutions of an import, weakest first.
const (
	// resolvesNowhere is an import whose names have no definition.
	resolvesNowhere resolution = iota
	// resolvesItself is an import whose names have definitions inside the import alone, as
	// typescript-language-server defines an import specifier as itself.
	resolvesItself
	// resolvesElsewhere is an import with a definition outside the import and outside the
	// target.
	resolvesElsewhere
	// resolvesInto is an import with a definition in the target.
	resolvesInto
)

// encloses reports whether outer contains inner, both spans of one file.
func encloses(outer, inner source.Span) bool {
	return outer.Start.Offset <= inner.Start.Offset && inner.End.Offset <= outer.End.Offset
}

// importers returns the imports of the workspace that import the target of req, for a server
// that declares [Server.Imports]. The outline engine of the language finds each import that
// writes the name of of, as its [engine.Relator] relates it, and the engine keeps an import whose
// resolution is [resolvesInto], by the rule of [Engine.importsInto]. The target is the file of
// the declaration of req, or else the scope of req, a file or a directory. The engine asks once
// the server has settled, as for every other relation. It asks again at the imports that
// resolve to no file while [Engine.rewarmed] reports true.
//
// The answer is resolved and partial, with the caveats of [settled] and [byName]. Each import
// that the server resolves to no file but its own counts in a caveat of its own.
//
// importers declines a request when the outline engine does not relate imports.
func (e *Engine) importers(
	ctx context.Context,
	held *session,
	req engine.Request,
	of sema.ID,
) (engine.Result[sema.Relation], error) {
	reads, relates := e.outliner.(engine.Relator)
	if !relates {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s: no outline engine reads the imports of the language", engine.ErrDecline, e.server.Name)
	}
	asked := req
	asked.Limit = 0
	found, err := reads.Relate(ctx, asked, of, sema.ImportedBy)
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}

	target := req.Scope
	if req.Declared.Path != "" {
		target = req.Declared.Path
	}
	_, caveats := settled(e.settle(ctx, held))
	outlines := newFinder(e, held)
	var out, undefined []sema.Relation
	for pending := found.Items; ; {
		var nowhere []sema.Relation
		for _, one := range pending {
			resolved, err := e.importsInto(ctx, held, outlines, one.At, target)
			switch {
			case err != nil:
				return engine.Result[sema.Relation]{}, err
			case resolved == resolvesInto:
				out = append(out, one)
			case resolved == resolvesItself:
				undefined = append(undefined, one)
			case resolved == resolvesNowhere:
				nowhere = append(nowhere, one)
			}
		}
		if len(nowhere) == 0 || !e.rewarmed(ctx, held) {
			undefined = append(undefined, nowhere...)
			break
		}
		pending = nowhere
	}
	slices.SortFunc(out, order)
	slices.SortFunc(undefined, order)

	caveats = append(caveats, byName)
	if len(undefined) > 0 {
		at := undefined[0].At
		caveats = append(caveats, trust.Caveat{
			Code: trust.CaveatUnsupported,
			Note: fmt.Sprintf("the server resolved %d of the imports that write the name to no other file, "+
				"such as the import at %s:%d", len(undefined), at.Path, at.Start.Line+1),
		})
	}
	return engine.Result[sema.Relation]{Items: out, Completeness: trust.ScopePartial, Caveats: caveats}, nil
}

// rewarmed waits [reasking] and reports whether the server of held has still run for less than
// its [Server.Resolving] when the wait ends, so that it can now resolve an import that it
// resolved to no file. It reports false at once for a server that has run longer, and when ctx
// ends during the wait.
func (e *Engine) rewarmed(ctx context.Context, held *session) bool {
	if time.Since(held.started)+reasking > e.server.Resolving {
		return false
	}
	pause := time.NewTimer(reasking)
	defer pause.Stop()
	select {
	case <-pause.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// importsInto returns the resolution of the import whose declaration spans at, against target,
// a file or a directory, by the rule of [Engine.resolvedAt].
//
// It asks at the imports of the outline of the file with that span first. One declaration can
// import more than one name, as from m import a, b does in Python, and the outline engine
// matched one of them. When the server resolves those names to the import itself, it asks at
// the imports whose spans contain at, such as the module of the import statement whose import
// specifier at is.
func (e *Engine) importsInto(
	ctx context.Context,
	held *session,
	outlines *finder,
	at source.Span,
	target source.Path,
) (resolution, error) {
	kept, err := outlines.file(ctx, at.Path)
	if err != nil {
		return resolvesNowhere, err
	}
	doc, err := e.open(ctx, held, at.Path)
	if err != nil {
		return resolvesNowhere, err
	}
	own, err := e.resolvedAt(ctx, held, doc, kept.symbols, target, func(s source.Span) bool { return s == at })
	if err != nil || own != resolvesItself {
		return own, err
	}
	return e.resolvedAt(ctx, held, doc, kept.symbols, target, func(s source.Span) bool {
		return s != at && encloses(s, at)
	})
}

// resolvedAt returns the strongest resolution of the definitions at the names of the imports of
// symbols, the declarations of doc, whose spans selected keeps. It asks in the order of symbols
// and stops at the first definition in target. A definition inside the span of the import that
// it asked about is the import itself.
//
// The request asks at the last character of the last occurrence of the name in the text of the
// declaration of the import. The last occurrence is the name of import port, whose keyword
// contains port too. jdtls returns no definition at the qualifier of a Java import, and every
// server measured returns the imported file at the end of the name. A name whose definition
// request fails has no definition, unless the context of the call ends.
func (e *Engine) resolvedAt(
	ctx context.Context,
	held *session,
	doc document,
	symbols []sema.Symbol,
	target source.Path,
	selected func(source.Span) bool,
) (resolution, error) {
	out := resolvesNowhere
	for _, one := range symbols {
		if one.Kind != sema.KindImport || !selected(one.Span) {
			continue
		}
		written := strings.LastIndex(doc.text(one.Span), one.Name)
		end := doc.mark(source.Position{Offset: one.Span.Start.Offset + written + len(one.Name) - 1})
		defined, failed := e.definedAt(ctx, held, doc, end)
		if ctx.Err() != nil {
			return resolvesNowhere, ctx.Err()
		}
		if failed != nil {
			continue
		}
		for _, d := range defined {
			p := e.pathOf(d.URI)
			switch {
			case p == doc.path && encloses(one.Span, doc.span(d.Range)):
				out = max(out, resolvesItself)
			case lang.Within(p, target):
				return resolvesInto, nil
			default:
				out = max(out, resolvesElsewhere)
			}
		}
	}
	return out, nil
}
