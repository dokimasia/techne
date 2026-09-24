// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package ruby declares the Ruby language:
//
//   - its extensions, project manifests, comment forms and test files
//   - its tree-sitter grammar and tags query
//   - ruby-lsp, its language server
//
// A composition root calls [Register], which adds Ruby to a registry and
// its engines to a catalogue.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/source, lang,
// lang/engines, lang/lsp, lang/treesitter and the tree-sitter grammar of
// Ruby. It does not import another language module.
package ruby
