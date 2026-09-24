// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package python declares the Python language:
//
//   - its extensions, project manifests, comment forms and test files
//   - its visibility rule, which PEP 8 states
//   - its tree-sitter grammar and tags query
//   - pyright, its language server
//
// A composition root calls [Register], which adds Python to a registry and
// its engines to a catalogue.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/sema, core/source, lang,
// lang/engines, lang/lsp, lang/treesitter and the tree-sitter grammar of
// Python. It does not import another language module.
package python
