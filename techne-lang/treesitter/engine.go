// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"fmt"
	"io/fs"

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
	tags     *ts.Query
}

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
	case d.Namespace == nil || d.Exported == nil:
		return nil, fmt.Errorf("treesitter: %q declares no conventions", d.Language)
	case g.Language == nil:
		return nil, fmt.Errorf("treesitter: %q supplies no grammar", d.Language)
	case g.Tags == "":
		return nil, fmt.Errorf("treesitter: %q supplies no tags query", d.Language)
	}

	q, qerr := ts.NewQuery(g.Language, g.Tags)
	if qerr != nil {
		return nil, fmt.Errorf("treesitter: %q tags query: %w", d.Language, *qerr)
	}
	return &Engine{fsys: fsys, declared: d, grammar: g, tags: q}, nil
}

// Close releases the compiled query. Calling it twice is safe.
func (e *Engine) Close() {
	if e.tags != nil {
		e.tags.Close()
		e.tags = nil
	}
}

// Name identifies this adapter in a provenance and a capability report.
func (*Engine) Name() string { return "treesitter" }

// Language is the one language this engine answers about.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity is [trust.Syntactic] for every role. A parser matched text,
// so a name resolved across files is coincidence and an empty answer
// never proves absence.
func (*Engine) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }

// Cost is [engine.CostParse]: one parse per file in scope.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostParse }
