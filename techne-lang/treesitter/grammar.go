// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import ts "github.com/tree-sitter/go-tree-sitter"

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
}
