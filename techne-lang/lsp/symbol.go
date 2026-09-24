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

// finder returns the declaration at a location that a server returned. It reads the
// declarations of each file once per call and keeps them for that call only, because the
// answers of a server change with the workspace.
//
// The declaration that a question is about comes from the document symbols of the server,
// because a request names the position of a name that the server reported. The declaration at
// a site or at a definition comes from the outline engine of the language when the engine has
// one, so the server does not open the file. A file outside the workspace has no outline of
// that engine, and its declarations come from the server.
type finder struct {
	engine  *Engine
	session *session
	// served are the outlines from the document symbols of the server, and parsed the
	// outlines from the outline engine of the language.
	served, parsed map[source.Path]outline
}

// outline is the document of one file and its declarations, both empty for a file that the
// engine does not read.
type outline struct {
	symbols []sema.Symbol
	// offered are the declarations of symbols that the file offers to the rest of a program:
	// no import, parameter, label or local declaration.
	offered []sema.Symbol
	doc     document
}

// outlined returns the outline of doc with symbols.
func outlined(doc document, symbols []sema.Symbol) outline {
	locals := sema.Locals(symbols, sema.Containers(symbols))
	var offered []sema.Symbol
	for i, one := range symbols {
		if engine.Bindings(0).Keeps(one.Kind, locals[i]) {
			offered = append(offered, one)
		}
	}
	return outline{symbols: symbols, offered: offered, doc: doc}
}

// newFinder returns a finder for one call on held.
func newFinder(e *Engine, held *session) *finder {
	return &finder{
		engine: e, session: held,
		served: map[source.Path]outline{}, parsed: map[source.Path]outline{},
	}
}

// file returns the outline of the file at p, from the outline engine of the language when the
// engine has one and p is in the workspace, and from the server otherwise. A file of another
// language, a file that [lang.Readable] refuses and a file outside the workspace larger than
// [lang.Largest] have an empty outline. Any other failure to read p is returned: the server
// named p, so the file must be readable.
func (f *finder) file(ctx context.Context, p source.Path) (outline, error) {
	if f.engine.outliner == nil || outside(p) {
		return f.opened(ctx, p)
	}
	if kept, known := f.parsed[p]; known {
		return kept, nil
	}
	if !lang.Claims(string(p), f.engine.declared.Extensions) {
		f.parsed[p] = outline{}
		return outline{}, nil
	}
	doc, err := f.engine.read(p)
	if refused(err) {
		f.parsed[p] = outline{}
		return outline{}, nil
	}
	if err != nil {
		return outline{}, err
	}
	answered, err := f.engine.outliner.Outline(ctx, engine.Request{Scope: p, Tests: true})
	if err != nil {
		return outline{}, fmt.Errorf("lsp: %s: the declarations of %s: %w", f.engine.server.Name, p, err)
	}
	kept := outlined(doc, answered.Items)
	f.parsed[p] = kept
	return kept, nil
}

// opened returns the outline of the file at p from the server, and checks p with
// [lang.Readable] before the server opens it.
func (f *finder) opened(ctx context.Context, p source.Path) (outline, error) {
	return f.symbolised(ctx, p, f.engine.open)
}

// walked returns the outline of the file at p from the server. p is a path from
// [Engine.walk], which [lang.Walk] has checked.
func (f *finder) walked(ctx context.Context, p source.Path) (outline, error) {
	return f.symbolised(ctx, p, f.engine.load)
}

// symbolised returns the kept outline of the file at p from the server, or reads the file
// with read, asks the server for its symbols and keeps the outline for the call.
func (f *finder) symbolised(
	ctx context.Context,
	p source.Path,
	read func(context.Context, *session, source.Path) (document, error),
) (outline, error) {
	if kept, known := f.served[p]; known {
		return kept, nil
	}
	if !lang.Claims(string(p), f.engine.declared.Extensions) {
		f.served[p] = outline{}
		return outline{}, nil
	}
	doc, err := read(ctx, f.session, p)
	if refused(err) {
		f.served[p] = outline{}
		return outline{}, nil
	}
	if err != nil {
		return outline{}, err
	}
	symbols, err := f.engine.symbols(ctx, f.session, doc)
	if err != nil {
		return outline{}, err
	}
	kept := outlined(doc, symbols)
	f.served[p] = kept
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

// within returns the innermost declaration of the file at p that contains the protocol
// position at and that the file offers to the rest of a program, and reports whether one does.
// A use inside the body of a function belongs to the function, not to a local variable that
// the body declares.
func (f *finder) within(ctx context.Context, p source.Path, at protocol.Position) (sema.Symbol, bool, error) {
	kept, err := f.file(ctx, p)
	if err != nil || len(kept.offered) == 0 {
		return sema.Symbol{}, false, err
	}
	found, known := innermost(kept.offered, kept.doc.position(at).Offset)
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
// and whether a declaration matches. It reads the symbols of each file through found, which
// keeps them for the rest of the call.
//
// The files are the file of [engine.Request.Declared] when req names one, and otherwise the
// files in paths without a test file unless req includes tests.
//
// A declaration with the ID is the match. Without one, the following steps apply in order. A
// step that selects exactly one declaration returns it:
//
//   - A declaration with the qualified name of the ID. A parser and a server can classify one
//     declaration under different kinds: metals reports a method of a Scala object where the
//     parser reports a function.
//   - A declaration of the kind of the ID whose qualified name and the qualified name of the ID
//     end in one another at a dot. A server can nest a declaration in a namespace or a package
//     that the parser does not qualify it by: csharp-ls nests a class in a file-scoped
//     namespace, and metals nests it in the packages of the package clause.
//   - A declaration whose name is the base of the qualified name of the ID. A parser and a
//     server can nest one declaration under different containers.
//
// Of two or more declarations with the ID, the match is the one whose span overlaps the
// declared span. Without an overlap it is the first. A step that selects two or more
// declarations returns the one whose span overlaps the declared span. Without an overlap the
// search ends without a match. When no step matches, the match is the declaration at the
// declared span. clangd lists no macro among its symbols and classifies a typedef of a struct
// as the struct.
func (e *Engine) declaring(
	ctx context.Context,
	found *finder,
	req engine.Request,
	of sema.ID,
	paths []source.Path,
) (sema.Symbol, document, bool, error) {
	declared := req.Declared
	if declared.Path != "" && lang.Claims(string(declared.Path), e.declared.Extensions) {
		return e.declaredAt(ctx, found, of, declared)
	}

	var exact, qualified, nested, based candidates
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
		e.candidates(kept, of, &exact, &qualified, &nested, &based)
		// The first declaration with the ID is the match, and no file after it is opened.
		if len(exact.symbols) > 0 {
			return exact.symbols[0], exact.docs[0], true, nil
		}
	}
	one, doc, known := matched(source.Span{}, exact, qualified, nested, based)
	return one, doc, known, nil
}

// declaredAt is [Engine.declaring] for a request that names the span of the declaration. It
// reads the symbols of the file of declared alone, and returns the declaration at declared
// when no symbol matches of.
func (e *Engine) declaredAt(
	ctx context.Context,
	found *finder,
	of sema.ID,
	declared source.Span,
) (sema.Symbol, document, bool, error) {
	kept, err := found.opened(ctx, declared.Path)
	if err != nil {
		return sema.Symbol{}, document{}, false, err
	}
	if len(kept.doc.content) == 0 {
		return sema.Symbol{}, document{}, false, nil
	}
	var exact, qualified, nested, based candidates
	e.candidates(kept, of, &exact, &qualified, &nested, &based)
	if one, doc, known := matched(declared, exact, qualified, nested, based); known {
		return one, doc, true, nil
	}
	return sema.Symbol{ID: of, Name: of.Base(), Span: declared}, kept.doc, true, nil
}

// candidates adds each declaration of kept to the step of [Engine.declaring] that selects it
// for of.
func (e *Engine) candidates(kept outline, of sema.ID, exact, qualified, nested, based *candidates) {
	unit := source.Path(e.declared.Namespace(string(kept.doc.path)))
	for _, one := range kept.symbols {
		switch {
		case one.ID == of:
			exact.add(one, kept.doc)
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

// matched returns the declaration that the steps of [Engine.declaring] select, in order, with
// the document of its file, and reports whether a step selects one. declared is the span that
// the request names, or the zero span.
func matched(declared source.Span, exact candidates, steps ...candidates) (sema.Symbol, document, bool) {
	if one, doc, known := exact.pick(declared); known {
		return one, doc, true
	}
	if len(exact.symbols) > 0 {
		return exact.symbols[0], exact.docs[0], true
	}
	for _, step := range steps {
		if one, doc, known := step.pick(declared); known {
			return one, doc, true
		}
		if len(step.symbols) > 1 {
			break
		}
	}
	return sema.Symbol{}, document{}, false
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

// pick returns the only declaration of c, or the only one whose span overlaps declared, and
// reports whether there is one.
func (c candidates) pick(declared source.Span) (sema.Symbol, document, bool) {
	if len(c.symbols) == 1 {
		return c.symbols[0], c.docs[0], true
	}
	at := -1
	for i, one := range c.symbols {
		if !overlaps(one.Span, declared) {
			continue
		}
		if at >= 0 {
			return sema.Symbol{}, document{}, false
		}
		at = i
	}
	if at < 0 {
		return sema.Symbol{}, document{}, false
	}
	return c.symbols[at], c.docs[at], true
}

// overlaps reports whether two spans of one file share a byte. The zero span overlaps nothing.
func overlaps(a, b source.Span) bool {
	return a.Path != "" && a.Path == b.Path &&
		a.Start.Offset < b.End.Offset && b.Start.Offset < a.End.Offset
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
