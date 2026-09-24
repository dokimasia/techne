// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Engine is the syntactic engine of one language. It reads files through an
// io/fs.FS rooted at the workspace. It is safe for concurrent use: every
// call takes its own parser, because a tree-sitter parser contains the
// state of one parse.
//
// The engine keeps the names, kinds and visibilities of the declarations of
// each file it parses. A search, a relation of imports and the lookup of a
// plan target read a file again only when its size or its modification time
// changed, or when it declares a name that the call selects.
type Engine struct {
	fsys     fs.FS
	declared lang.Declaration
	grammar  Grammar
	// tags is the compiled query of each grammar. A query compiles against
	// one grammar, so a language with a dialect has one query per grammar.
	tags  map[*ts.Language]*ts.Query
	scans scans
}

// ErrUnknownCapture reports a query with a definition capture that no kind
// covers.
var ErrUnknownCapture = errors.New("treesitter: unknown definition capture")

// New compiles the tags query of g for each of its grammars and returns the
// engine. It returns an error for a nil fsys, an incomplete declaration, a
// grammar that lacks a language or a query, and a query that does not
// compile. It returns [ErrUnknownCapture] for a definition capture that no
// kind covers, because such a capture matches and is dropped.
//
// The caller calls Close when it no longer needs the engine.
func New(fsys fs.FS, d lang.Declaration, g Grammar) (*Engine, error) {
	switch {
	case fsys == nil:
		return nil, fmt.Errorf("treesitter: no filesystem to read from")
	case d.Language == "":
		return nil, fmt.Errorf("treesitter: declaration has no language")
	case len(d.Extensions) == 0:
		return nil, fmt.Errorf("treesitter: %q declares no extension", d.Language)
	case d.Namespace == nil || d.Visibility == nil:
		return nil, fmt.Errorf("treesitter: %q declares no Namespace or no Visibility", d.Language)
	case g.Language == nil:
		return nil, fmt.Errorf("treesitter: %q supplies no grammar", d.Language)
	case g.Tags == "":
		return nil, fmt.Errorf("treesitter: %q supplies no tags query", d.Language)
	}

	e := &Engine{
		fsys: fsys, declared: d, grammar: g, tags: map[*ts.Language]*ts.Query{},
		scans: scans{files: map[source.Path]scan{}},
	}
	for _, grammar := range g.each() {
		if grammar == nil {
			e.Close()
			return nil, fmt.Errorf("treesitter: %q declares a dialect without a grammar", d.Language)
		}
		q, qerr := ts.NewQuery(grammar, g.Tags)
		if qerr != nil {
			e.Close()
			return nil, fmt.Errorf("treesitter: %q tags query: %w", d.Language, *qerr)
		}
		for _, name := range q.CaptureNames() {
			if !strings.HasPrefix(name, DefinitionPrefix) {
				continue
			}
			if _, known := KindOf(Capture(name)); !known {
				q.Close()
				e.Close()
				return nil, fmt.Errorf("%w: %q captures @%s", ErrUnknownCapture, d.Language, name)
			}
		}
		e.tags[grammar] = q
	}
	return e, nil
}

// Close releases the compiled queries and the kept declarations. It is safe
// to call more than once.
func (e *Engine) Close() {
	for _, q := range e.tags {
		q.Close()
	}
	clear(e.tags)
	e.scans.mu.Lock()
	clear(e.scans.files)
	e.scans.mu.Unlock()
}

// Name returns "treesitter/" followed by the language, so the engines of
// two languages have distinct names in one catalogue.
func (e *Engine) Name() string { return "treesitter/" + string(e.declared.Language) }

// Language returns the language of the engine.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity returns [trust.Syntactic] for every role.
func (*Engine) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }

// Cost returns [engine.CostParse] for every role: one parse per file in
// scope.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostParse }
