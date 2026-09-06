// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"path"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// Grammar is what a language module supplies so this engine can answer
// about its language.
//
// The zero Grammar declares nothing and [New] refuses it.
type Grammar struct {
	// Language is the compiled grammar, from the language's own
	// tree-sitter binding.
	Language *ts.Language

	// Tags is the query source, written in the convention the grammars'
	// own tags queries use, so a module vendors its query unchanged.
	// See [Capture].
	Tags string

	// Dialects are the grammars for extensions the main one cannot
	// parse, by extension.
	//
	// One language is sometimes two grammars. TypeScript is: .tsx is
	// TypeScript with JSX in it, and the plain grammar does not parse
	// it — measured, a component file comes back as a tree with an
	// error in it, so every declaration under the error is lost. The
	// same tags query compiles against both, which is what makes this a
	// second grammar rather than a second language: a declaration in a
	// .tsx file is TypeScript, and a rename that crossed the two would
	// otherwise be two languages and refuse itself.
	Dialects map[string]*ts.Language
}

// For returns the grammar that parses one file.
func (g Grammar) For(p string) *ts.Language {
	if held, dialect := g.Dialects[path.Ext(p)]; dialect {
		return held
	}
	return g.Language
}

// each yields every grammar this declares, the main one first, so a
// caller that must do something per grammar cannot miss a dialect.
func (g Grammar) each() []*ts.Language {
	out := []*ts.Language{g.Language}
	for _, held := range g.Dialects {
		out = append(out, held)
	}
	return out
}
