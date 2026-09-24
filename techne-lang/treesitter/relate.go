// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

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
	"go.dokimi.dev/techne/lang"
)

// matchedImport is the note of the caveat of every relation answer.
const matchedImport = "a parser matched the name an import is written under, not what it resolves to"

// separators are the characters that separate the segments of an import in
// the ten languages: a slash, a backslash, a colon and a dot.
const separators = `/\:.`

// Relate returns the import relations of a declaration: the files that
// import it, for [sema.ImportedBy], and the imports of its file, for
// [sema.Imports]. The tags query of every language captures imports as
// [sema.KindImport], so these relations do not need name binding.
//
// Relate returns a result with Skipped set for a scope without a file of
// the language. For any other relation kind it returns
// [engine.ErrDecline], because calls, references and implementations need
// name binding and no query captures embedding.
//
// A name matches an import, without the quotes or angle brackets of the
// import, in these forms:
//
//   - the whole import, such as encoding/json or stdio.h
//   - the end of the import after a separator, such as json or util.List
//   - the end of the import without its extension, such as stdio for
//     <stdio.h> or store for ./store.js
//
// The last element of a path loses any extension. An import without a
// slash loses only an extension of the language, because the dots of
// java.util.List separate segments. An import renamed at its use site
// matches as written. ImportedBy returns one relation for each line that
// imports the name.
func (e *Engine) Relate(
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
		return result[sema.Relation](nil, files, matchedImport), nil
	}
	if kind != sema.Imports && kind != sema.ImportedBy {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s reads imports, and %s needs name binding", engine.ErrDecline, e.Name(), kind)
	}

	name := of.Name()
	selected := func(d named) bool { return d.kind == sema.KindImport || d.qualified == name }
	found, err := parse(ctx, e, files.Read, selected,
		func(p source.Path, content []byte, declared []sema.Symbol) []sema.Relation {
			return e.edges(declared, content, p, name, kind)
		})
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}
	out := slices.Concat(found...)
	slices.SortFunc(out, ordered)
	return result(out, files, matchedImport), nil
}

// edges returns the import relations of kind in one file.
func (e *Engine) edges(
	declared []sema.Symbol,
	content []byte,
	p source.Path,
	name string,
	kind sema.RelationKind,
) []sema.Relation {
	// Imports lists the imports of the file that declares the qualified name.
	if kind == sema.Imports && !slices.ContainsFunc(declared, func(s sema.Symbol) bool { return s.ID.Name() == name }) {
		return nil
	}

	// One statement can import a name twice, as TypeScript's
	// `import * as fs from "fs"` declares the path and the binding. Imports
	// keeps each name once, and ImportedBy keeps each line once.
	names, lines := map[string]bool{}, map[int]bool{}

	var out []sema.Relation
	for _, one := range declared {
		if one.Kind != sema.KindImport {
			continue
		}
		switch kind {
		case sema.ImportedBy:
			if !e.imports(one.Name, name) || lines[one.Span.Start.Line] {
				continue
			}
			lines[one.Span.Start.Line] = true
		default:
			if names[one.Name] {
				continue
			}
			names[one.Name] = true
		}
		out = append(out, sema.Relation{
			Kind: kind,
			To:   far(one, p, kind, e.declared.Language),
			At:   one.Span,
			Via:  strings.TrimSpace(lang.LineAt(content, one.Span.Start.Offset)),
		})
	}
	return out
}

// far returns the declaration at the far end of an import relation: the
// importing file for [sema.ImportedBy], and the import for [sema.Imports].
func far(one sema.Symbol, p source.Path, kind sema.RelationKind, l source.Language) sema.Symbol {
	if kind == sema.Imports {
		return one
	}
	return sema.Symbol{
		ID:       sema.NewID(l, p, string(p), sema.KindFile),
		Name:     string(p),
		Kind:     sema.KindFile,
		Language: l,
		Span:     source.Span{Path: p},
	}
}

// imports reports whether name matches the import written as written, by
// the forms that [Engine.Relate] lists.
func (e *Engine) imports(written, name string) bool {
	written, name = bare(written), bare(name)
	if name == "" {
		return false
	}
	stem := e.stem(written)
	if extension := written[len(stem):]; extension != "" && strings.HasSuffix(name, extension) {
		return ends(written, name)
	}
	return ends(stem, name)
}

// stem returns written without the extension of its last element. It
// removes the extension of a path whatever it is, and the extension of an
// import without a slash only when the language claims it.
func (e *Engine) stem(written string) string {
	last := written[strings.LastIndexAny(written, `/\`)+1:]
	if last == written && !lang.Claims(written, e.declared.Extensions) {
		return written
	}
	return strings.TrimSuffix(written, path.Ext(last))
}

// ends reports whether name equals written, or is the end of written after
// a separator.
func ends(written, name string) bool {
	rest, found := strings.CutSuffix(written, name)
	return found && (rest == "" || strings.IndexByte(separators, rest[len(rest)-1]) >= 0)
}

// bare returns an import without the quotes or angle brackets around it.
func bare(written string) string {
	return strings.Trim(written, `"'<>`)
}

// ordered orders relations by path, then offset, then the name of the far
// end.
func ordered(a, b sema.Relation) int {
	return cmp.Or(
		cmp.Compare(a.At.Path, b.At.Path),
		cmp.Compare(a.At.Start.Offset, b.At.Start.Offset),
		cmp.Compare(a.To.Name, b.To.Name),
	)
}
