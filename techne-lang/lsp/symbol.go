// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// symbols returns the declarations of doc as the server reports them with
// textDocument/documentSymbol. It records the position of each name in doc.names. The server
// must have a buffer of doc, which [Engine.open] and [Engine.load] ensure.
//
// A reply of DocumentSymbol entries keeps the nesting that the server reported, and each
// declaration is qualified by the symbols that contain it. A reply of SymbolInformation
// entries gives the name of each container and no tree. Its declarations are qualified by
// that name and have no parent. A symbol whose kind declares nothing, such as a string in a
// JSON document, is dropped, and its children take its place.
func (e *Engine) symbols(ctx context.Context, held *session, doc document) ([]sema.Symbol, error) {
	if !provides(held.capable.DocumentSymbolProvider) {
		return nil, e.unsupported("textDocument/documentSymbol")
	}
	answered, err := held.asks.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(doc.path))},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: symbols of %s: %w", e.server.Name, doc.path, err)
	}

	unit := source.Path(e.declared.Namespace(string(doc.path)))
	var out []sema.Symbol
	switch reported := answered.(type) {
	case protocol.DocumentSymbolSlice:
		e.nest(&out, reported, doc, unit, "", "")
	case protocol.SymbolInformationSlice:
		e.list(&out, reported, doc, unit)
	}
	return out, nil
}

// nest appends the declarations of a DocumentSymbol tree to into, depth first, with parent as
// the parent of the top level and container as its qualified name.
//
// A symbol is no container when the parser does not declare one for it. Its children then take
// its place, with its parent and its container. That applies to a symbol whose kind declares
// nothing, to a symbol of the kind File and to a symbol that [anonymous] reports. An impl
// block that rust-analyzer reports is an implementation named for the type it implements, as
// [implemented] reads it.
func (e *Engine) nest(
	into *[]sema.Symbol,
	reported []protocol.DocumentSymbol,
	doc document,
	unit source.Path,
	parent sema.ID,
	container string,
) {
	for _, one := range reported {
		kind, declares := KindOf(one.Kind)
		name := trimmed(one.Name)
		if implementing, is := implemented(one); is {
			kind, name, declares = sema.KindImplementation, implementing, true
		}
		if !declares || one.Kind == protocol.SymbolKindFile || anonymous(one.Name) {
			e.nest(into, one.Children, doc, unit, parent, container)
			continue
		}
		within := qualified(kind, name, container, one.Name)
		id := sema.NewID(e.declared.Language, unit, within, kind)
		span := doc.span(one.Range)
		doc.names[span.Start.Offset] = one.SelectionRange.Start

		*into = append(*into, sema.Symbol{
			ID:         id,
			Name:       name,
			Kind:       kind,
			Language:   e.declared.Language,
			Span:       span,
			Parent:     parent,
			Visibility: e.declared.Visibility(name),
			Signature:  signature(name, one.SelectionRange, optional(one.Detail), doc),
			Snippet:    doc.text(span),
		})
		e.nest(into, one.Children, doc, unit, id, within)
	}
}

// list appends the declarations of a SymbolInformation list to into. An entry in another
// file than doc is dropped, because a server may list the declarations of files it read for
// doc.
func (e *Engine) list(
	into *[]sema.Symbol,
	reported []protocol.SymbolInformation,
	doc document,
	unit source.Path,
) {
	for _, one := range reported {
		kind, declares := KindOf(one.Kind)
		if !declares || e.pathOf(one.Location.URI) != doc.path {
			continue
		}
		name := trimmed(one.Name)
		span := doc.span(one.Location.Range)
		within := qualified(kind, name, optional(one.ContainerName), one.Name)
		*into = append(*into, sema.Symbol{
			ID:         sema.NewID(e.declared.Language, unit, within, kind),
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

// signature returns the line on which the name of a declaration starts, cut by [lang.Excerpt]
// around the name, without leading and trailing white space. For an empty line it returns
// the name and the detail the server reported, such as the parameters and results gopls
// reports.
func signature(name string, at protocol.Range, detail string, doc document) string {
	named := source.Span{Start: doc.position(at.Start)}
	if line := doc.sourceLine(named); line != "" {
		return line
	}
	return strings.TrimSpace(name + " " + detail)
}

// optional returns the text of a field that a server may leave out, such as the detail of a
// symbol, or the empty string for none.
func optional(reported *string) string {
	if reported == nil {
		return ""
	}
	return *reported
}

// trimmed returns the plain name of a declaration from the name a server reported. gopls
// reports a method as (*Store).Get, and jdtls reports helper() in an outline and helper() : int
// in a call hierarchy. trimmed returns Get and helper: it removes the parameters from the first
// parenthesis after the start of the name, then the qualifier up to the last dot.
func trimmed(name string) string {
	if at := strings.Index(name, "("); at > 0 {
		name = name[:at]
	}
	if at := strings.LastIndex(name, "."); at >= 0 {
		name = name[at+1:]
	}
	return strings.TrimSpace(name)
}

// qualifier returns the type that the name a server reported writes before the plain name,
// without its pointer, its parentheses and its type arguments: Store for (*Store).Get, and
// Cache for (*Cache[T]).Get. It returns the empty string for a name without one.
func qualifier(reported string) string {
	if at := strings.Index(reported, "("); at > 0 {
		reported = reported[:at]
	}
	at := strings.LastIndex(reported, ".")
	if at < 0 {
		return ""
	}
	out := strings.Trim(reported[:at], "(*)")
	if cut := strings.IndexByte(out, '['); cut >= 0 {
		out = out[:cut]
	}
	return strings.TrimSpace(out)
}

// implemented returns the name of the type that an impl block implements, from the Object
// symbol that rust-analyzer reports for the block. rust-analyzer labels a block impl Store or
// impl Getter for Store, with the type as the source writes it. implemented returns Store for
// Store and for Store<T>, which is the name that the parser gives the block. It reports false
// for another symbol, and for a type that is not a name with optional type arguments, such as
// &Store. The parser does not declare a block for such a type.
func implemented(one protocol.DocumentSymbol) (string, bool) {
	if one.Kind != protocol.SymbolKindObject {
		return "", false
	}
	rest, labelled := strings.CutPrefix(one.Name, "impl ")
	if !labelled {
		return "", false
	}
	if _, after, trait := strings.Cut(rest, " for "); trait {
		rest = after
	}
	if at := strings.IndexByte(rest, '<'); at >= 0 {
		rest = rest[:at]
	}
	return rest, identifier(rest)
}

// anonymous reports whether name is the name that a server gives a declaration without one.
// The TypeScript server gives an anonymous function the name <function>, an anonymous class
// <class>, and a declaration without a name of its own <unknown>. A function passed to a call
// gets the name of the call, such as describe("x") callback.
func anonymous(name string) bool {
	return strings.HasPrefix(name, "<") && strings.HasSuffix(name, ">") || strings.HasSuffix(name, " callback")
}

// identifier reports whether name is a nonempty run of letters, digits and underscores that
// does not start with a digit.
func identifier(name string) bool {
	for i, r := range name {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return name != ""
}

// qualified returns the qualified name of a declaration of kind, named name and reported as
// reported, inside the declaration whose qualified name is container. A declaration at the top
// level takes the qualifier of its reported name, as gopls reports a method of Store at the
// top level as (*Store).Get. An import is not qualified, because its name is a path.
func qualified(kind sema.Kind, name, container, reported string) string {
	switch {
	case kind == sema.KindImport:
		return name
	case container == "":
		container = qualifier(reported)
	}
	return sema.Qualify(container, name)
}

// innermost returns the smallest declaration whose span contains offset, and reports whether
// one does. A span contains its end offset, so the position right after a declaration belongs
// to it.
func innermost(symbols []sema.Symbol, offset int) (sema.Symbol, bool) {
	var found sema.Symbol
	var known bool
	for _, one := range symbols {
		if one.Span.Start.Offset > offset || one.Span.End.Offset < offset {
			continue
		}
		if !known || size(one.Span) < size(found.Span) {
			found, known = one, true
		}
	}
	return found, known
}

// size returns the number of bytes that s covers.
func size(s source.Span) int { return s.End.Offset - s.Start.Offset }

// finder returns the declaration at a location that a server returned. It reads the symbols
// of each file once per call and keeps them for that call only, because the answers of a
// server change with the workspace.
type finder struct {
	engine   *Engine
	session  *session
	outlines map[source.Path]outline
}

// outline is the document of one file and its declarations, both empty for a file that the
// engine does not read.
type outline struct {
	symbols []sema.Symbol
	doc     document
}

// newFinder returns a finder for one call on held.
func newFinder(e *Engine, held *session) *finder {
	return &finder{engine: e, session: held, outlines: map[source.Path]outline{}}
}

// file returns the outline of the file at p. A file of another language, a file that
// [lang.Readable] refuses and a file outside the workspace larger than [lang.Largest] have an
// empty outline. Any other failure to read p is returned: the server named p, so the file must
// be readable.
func (f *finder) file(ctx context.Context, p source.Path) (outline, error) {
	return f.outlined(ctx, p, f.engine.open)
}

// walked returns the outline of the file at p, a path from [Engine.walk], which [lang.Walk]
// has checked.
func (f *finder) walked(ctx context.Context, p source.Path) (outline, error) {
	return f.outlined(ctx, p, f.engine.load)
}

// outlined returns the kept outline of the file at p, or reads the file with read, asks the
// server for its symbols and keeps the outline for the call.
func (f *finder) outlined(
	ctx context.Context,
	p source.Path,
	read func(context.Context, *session, source.Path) (document, error),
) (outline, error) {
	if kept, known := f.outlines[p]; known {
		return kept, nil
	}
	if !lang.Claims(string(p), f.engine.declared.Extensions) {
		f.outlines[p] = outline{}
		return outline{}, nil
	}
	doc, err := read(ctx, f.session, p)
	if refused(err) {
		f.outlines[p] = outline{}
		return outline{}, nil
	}
	if err != nil {
		return outline{}, err
	}
	symbols, err := f.engine.symbols(ctx, f.session, doc)
	if err != nil {
		return outline{}, err
	}
	kept := outline{symbols: symbols, doc: doc}
	f.outlines[p] = kept
	return kept, nil
}

// at returns the innermost declaration of the file at p that contains the protocol position
// at, and reports whether one does.
func (f *finder) at(ctx context.Context, p source.Path, at protocol.Position) (sema.Symbol, bool, error) {
	kept, err := f.file(ctx, p)
	if err != nil || len(kept.symbols) == 0 {
		return sema.Symbol{}, false, err
	}
	found, known := innermost(kept.symbols, kept.doc.position(at).Offset)
	return found, known, nil
}

// refused reports whether err is a [lang.LargeError] or a [lang.GeneratedError]: a file that
// no engine reads.
func refused(err error) bool {
	_, large := errors.AsType[lang.LargeError](err)
	_, generated := errors.AsType[lang.GeneratedError](err)
	return large || generated
}

// declaring returns the declaration that an ID identifies, the document of the file it is in,
// and whether a declaration matches. It reads the symbols of each file in paths through found,
// which keeps them for the rest of the call, and skips a test file unless req includes tests.
//
// A declaration with the ID is the match. Without one, the following steps apply in order, and
// a step that selects exactly one declaration returns it:
//
//   - A declaration with the qualified name of the ID, because a parser and a server can
//     classify one declaration under different kinds: metals reports a method of a Scala
//     object where the parser reports a function.
//   - A declaration of the kind of the ID whose qualified name and the qualified name of the ID
//     end in one another at a dot, because a server can nest a declaration in a namespace or a
//     package that the parser does not qualify it by: csharp-ls nests a class in a file-scoped
//     namespace, and metals nests it in the packages of the package clause.
//   - A declaration whose name is the base of the qualified name of the ID, because a parser
//     and a server can nest one declaration under different containers.
//
// A step that selects two or more declarations ends the search without a match.
func (e *Engine) declaring(
	ctx context.Context,
	found *finder,
	req engine.Request,
	of sema.ID,
	paths []source.Path,
) (sema.Symbol, document, bool, error) {
	var qualified, nested, based candidates
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return sema.Symbol{}, document{}, false, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		kept, err := found.walked(ctx, p)
		if err != nil {
			return sema.Symbol{}, document{}, false, err
		}
		unit := source.Path(e.declared.Namespace(string(p)))
		for _, one := range kept.symbols {
			switch {
			case one.ID == of:
				return one, kept.doc, true, nil
			case one.ID.Name() == of.Name():
				qualified.add(one, kept.doc)
			case sema.NewID(e.declared.Language, unit, of.Name(), one.Kind) == of &&
				dotted(one.ID.Name(), of.Name()):
				nested.add(one, kept.doc)
			case one.Name == of.Base():
				based.add(one, kept.doc)
			}
		}
	}
	for _, step := range []candidates{qualified, nested, based} {
		if len(step.symbols) > 1 {
			break
		}
		if len(step.symbols) == 1 {
			return step.symbols[0], step.docs[0], true, nil
		}
	}
	return sema.Symbol{}, document{}, false, nil
}

// candidates are the declarations that one step of [Engine.declaring] selects, with the
// document of the file of each.
type candidates struct {
	symbols []sema.Symbol
	docs    []document
}

// add appends one declaration and the document of its file.
func (c *candidates) add(one sema.Symbol, doc document) {
	c.symbols, c.docs = append(c.symbols, one), append(c.docs, doc)
}

// dotted reports whether one of two qualified names ends in the other at a dot, as
// MediaBrowser.Controller.MediaEncoding.EncodingHelper ends in EncodingHelper.
func dotted(a, b string) bool {
	return strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

// naming returns the protocol position of the name of a declaration, which is the position a
// request about the declaration names.
//
// It returns the start of the selection range that the server reported. A server that
// replied with SymbolInformation reports none, and naming then returns the first occurrence
// of the name as a whole word in the source of the declaration, or the start of the
// declaration when the name does not occur.
func naming(doc document, of sema.Symbol) protocol.Position {
	if at, reported := doc.names[of.Span.Start.Offset]; reported {
		return at
	}
	if at := lang.Worded(doc.text(of.Span), of.Name); at >= 0 {
		return doc.mark(source.Position{Offset: of.Span.Start.Offset + at})
	}
	return doc.mark(of.Span.Start)
}
