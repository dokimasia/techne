// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package golang declares the Go language:
//
//   - its extension, project manifests, comment forms and test files
//   - its packages, which are directories, and its visibility rule
//   - its tree-sitter grammar and tags query
//   - gopls, its language server
//
// A composition root calls [Register], which adds Go to a registry and its
// engines to a catalogue. For a workspace on disk, Register also adds the
// type checker of package checker.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/sema, core/source, lang,
// lang/engines, lang/lsp, lang/treesitter, lang/go/checker and the
// tree-sitter grammar of Go. It does not import another language module.
package golang
