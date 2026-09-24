// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"path"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// Grammar is what a language module supplies to the engine. [New] refuses
// the zero value.
type Grammar struct {
	// Language is the compiled grammar from the tree-sitter binding of the
	// language.
	Language *ts.Language

	// Tags is the source of the tags query, in the capture convention that
	// [Capture] documents.
	Tags string

	// Dialects maps each extension that Language cannot parse to the grammar
	// that parses it, such as ".tsx" to the TSX grammar in TypeScript. Tags
	// compiles against every dialect.
	Dialects map[string]*ts.Language
}

// For returns the grammar that parses the file at p.
func (g Grammar) For(p string) *ts.Language {
	if dialect, ok := g.Dialects[path.Ext(p)]; ok {
		return dialect
	}
	return g.Language
}

// each returns every grammar of g, Language first.
func (g Grammar) each() []*ts.Language {
	out := []*ts.Language{g.Language}
	for _, dialect := range g.Dialects {
		out = append(out, dialect)
	}
	return out
}
