// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Relate reports which files import a name, and what a file imports.
//
// # Only the edges a query already captures
//
// Every language module's tags query captures what a file brings into
// scope, as [sema.KindImport]. Those two directions follow from it
// without resolving anything, so a parser can answer them and a caller
// with no language server installed can still ask what depends on what.
//
// Every other direction is declined. Calls, references and implements
// are binding, which is what a language server is for; embedding is not
// captured by any query, so there is nothing here to read it from.
// Answering none would be a claim that a file imports nothing.
//
// # What the answer is worth
//
// A name matched as text. Two packages whose last segment is the same
// are one name here, and an import renamed at its use site is the name
// as written. That is what [trust.Syntactic] means, and the caveat says
// so.
func (e *Engine) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	if kind != sema.Imports && kind != sema.ImportedBy {
		return engine.Result[sema.Relation]{}, fmt.Errorf(
			"%w: %s reads imports, and %s is a binding this tier does not have",
			engine.ErrDecline, e.Name(), kind)
	}

	paths, err := lang.FilesIn(e.fsys, req.Scope, e.declared.Extensions)
	if err != nil {
		return engine.Result[sema.Relation]{}, err
	}

	name := of.Name()
	var out []sema.Relation
	var unread []source.Path
	read := 0
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return engine.Result[sema.Relation]{}, err
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		if unreadable := lang.Readable(e.fsys, p); unreadable != nil {
			if _, large := errors.AsType[lang.LargeError](unreadable); !large {
				return engine.Result[sema.Relation]{}, unreadable
			}
			unread = append(unread, p)
			continue
		}
		content, readErr := fs.ReadFile(e.fsys, string(p))
		if readErr != nil {
			return engine.Result[sema.Relation]{}, fmt.Errorf("treesitter: read %s: %w", p, readErr)
		}
		declared, outlineErr := e.declarations(p, content)
		if outlineErr != nil {
			return engine.Result[sema.Relation]{}, outlineErr
		}
		read++

		out = append(out, e.edges(declared, content, p, name, kind)...)
	}

	slices.SortFunc(out, ordered)
	caveats := []trust.Caveat{{
		Code: trust.CaveatDynamic,
		Note: "a parser matched the name an import is written under, " +
			"which is not the same as what it resolves to",
	}}
	covered := trust.ScopeTotal
	if len(unread) > 0 {
		covered = trust.ScopePartial
		caveats = append(caveats, trust.Caveat{
			Code:  trust.CaveatUnread,
			Note:  "past the size an engine parses, so these import nothing here",
			Paths: unread,
		})
	}
	return engine.Result[sema.Relation]{
		Items:        out,
		Skipped:      read == 0,
		Completeness: covered,
		Caveats:      caveats,
	}, nil
}

// edges reads one file's imports as edges in whichever direction was
// asked for.
func (e *Engine) edges(
	declared []sema.Symbol,
	content []byte,
	p source.Path,
	name string,
	kind sema.RelationKind,
) []sema.Relation {
	// Imports is about the file holding the declaration asked about, so
	// a file holding none of it has nothing to say.
	if kind == sema.Imports && !declaring(declared, name) {
		return nil
	}

	// A name brought in twice is one import. A language that binds a
	// module under its own name captures the path and the binding
	// separately, and both read as the same word: import * as fs from
	// "fs" would otherwise report fs twice.
	seen := map[string]bool{}

	var out []sema.Relation
	for _, one := range declared {
		if one.Kind != sema.KindImport {
			continue
		}
		if kind == sema.ImportedBy && !imports(one.Name, name) {
			continue
		}
		if kind == sema.Imports {
			if seen[one.Name] {
				continue
			}
			seen[one.Name] = true
		}
		out = append(out, sema.Relation{
			Kind: kind,
			To:   far(one, p, kind, e.declared.Language),
			At:   one.Span,
			Via:  written(content, one.Span),
		})
	}
	return out
}

// far is the declaration at the other end of an import edge.
//
// Which end that is depends on the direction. What imports this is
// answered by the file that does, because a caller asking who depends on
// a package wants the files rather than the import statements. What this
// imports is answered by the import itself, which is the name brought
// into scope.
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

// declaring reports whether a file declares a name.
func declaring(held []sema.Symbol, name string) bool {
	for _, one := range held {
		if one.Name == name {
			return true
		}
	}
	return false
}

// imports reports whether an import written one way names the same thing
// a caller asked about.
//
// A caller asks for what it reads, which is the last segment of a path
// as often as the whole of it: fmt, encoding/json, java.util.List,
// ./relative/module. Both are accepted, and the caveat states that
// matching text is all this did.
func imports(written, name string) bool {
	if name == "" {
		return false
	}
	held := strings.Trim(written, `"'`)
	if held == name {
		return true
	}
	return segment(held) == segment(name)
}

// segment is the last part of a path written with any of the separators
// the ten languages use.
func segment(held string) string {
	if at := strings.LastIndexAny(held, "/.\\"); at >= 0 && at+1 < len(held) {
		return held[at+1:]
	}
	return held
}

// written is the source an edge was written on.
func written(content []byte, at source.Span) string {
	from := at.Start.Offset
	if from < 0 || from > len(content) {
		return ""
	}
	start := strings.LastIndexByte(string(content[:from]), '\n') + 1
	end := strings.IndexByte(string(content[start:]), '\n')
	if end < 0 {
		return strings.TrimSpace(string(content[start:]))
	}
	return strings.TrimSpace(string(content[start : start+end]))
}

// ordered sorts edges by where they were written, so the same question
// answers the same way twice.
func ordered(a, b sema.Relation) int {
	if by := strings.Compare(string(a.At.Path), string(b.At.Path)); by != 0 {
		return by
	}
	if by := a.At.Start.Offset - b.At.Start.Offset; by != 0 {
		return by
	}
	return strings.Compare(a.To.Name, b.To.Name)
}
