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

// Engine answers about one language by parsing it.
//
// It holds a compiled query and is safe for concurrent use. A parser is
// taken per call rather than shared, because a tree-sitter parser holds
// the state of one parse.
//
// Files are read through an [io/fs.FS] rooted at the workspace, so paths
// crossing a port stay relative and a caller can serve a tree that is
// not on disk.
type Engine struct {
	fsys     fs.FS
	declared lang.Declaration
	grammar  Grammar
	// tags is the compiled query per grammar. A query is compiled
	// against one grammar and cannot be used with another, so a language
	// that declares a dialect has one for each.
	tags map[*ts.Language]*ts.Query
}

// ErrUnknownCapture reports a query naming a definition capture the
// vocabulary does not carry.
var ErrUnknownCapture = errors.New("treesitter: unknown definition capture")

// New compiles a grammar's queries and returns the engine serving them.
//
// A query that does not compile is a mistake in a language module.
// Failing here rather than returning no results at run time is the
// difference between a caught bug and a language that silently answers
// nothing.
//
// The caller closes the engine when it is finished with it.
func New(fsys fs.FS, d lang.Declaration, g Grammar) (*Engine, error) {
	switch {
	case fsys == nil:
		return nil, fmt.Errorf("treesitter: no filesystem to read from")
	case d.Language == "":
		return nil, fmt.Errorf("treesitter: declaration names no language")
	case len(d.Extensions) == 0:
		return nil, fmt.Errorf("treesitter: %q declares no extension", d.Language)
	case d.Namespace == nil || d.Visibility == nil:
		return nil, fmt.Errorf("treesitter: %q declares no conventions", d.Language)
	case g.Language == nil:
		return nil, fmt.Errorf("treesitter: %q supplies no grammar", d.Language)
	case g.Tags == "":
		return nil, fmt.Errorf("treesitter: %q supplies no tags query", d.Language)
	}

	held := &Engine{fsys: fsys, declared: d, grammar: g, tags: map[*ts.Language]*ts.Query{}}
	for _, one := range g.each() {
		if one == nil {
			held.Close()
			return nil, fmt.Errorf("treesitter: %q declares a dialect with no grammar", d.Language)
		}
		q, qerr := ts.NewQuery(one, g.Tags)
		if qerr != nil {
			held.Close()
			return nil, fmt.Errorf("treesitter: %q tags query: %w", d.Language, *qerr)
		}
		// A query naming a definition capture the vocabulary does not
		// carry would match and then be dropped, so the pattern would
		// find nothing and say nothing. That is the hardest failure to
		// notice in a system whose job includes reporting that it found
		// nothing, so it is refused here instead.
		for _, name := range q.CaptureNames() {
			if !strings.HasPrefix(name, DefinitionPrefix) {
				continue
			}
			if _, known := KindOf(Capture(name)); !known {
				q.Close()
				held.Close()
				return nil, fmt.Errorf("%w: %q captures @%s, which no kind carries",
					ErrUnknownCapture, d.Language, name)
			}
		}
		held.tags[one] = q
	}
	return held, nil
}

// Close releases the compiled queries. Calling it twice is safe.
func (e *Engine) Close() {
	for _, q := range e.tags {
		q.Close()
	}
	clear(e.tags)
}

// Name identifies this engine in a provenance and a capability report.
//
// One adapter serves every grammar, so the name carries the language it
// was built for. Without it five instances would share one name, the
// catalogue would refuse all but the first, and a provenance would not
// say which answered.
func (e *Engine) Name() string { return "treesitter/" + string(e.declared.Language) }

// Language is the one language this engine answers about.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity is [trust.Syntactic] for every role. A parser matched text,
// so a name resolved across files is coincidence and an empty answer
// never proves absence.
func (*Engine) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }

// Cost is [engine.CostParse]: one parse per file in scope.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostParse }
