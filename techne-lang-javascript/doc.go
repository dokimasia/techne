// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package javascript declares the JavaScript language:
//
//   - its extensions, project manifests and comment forms
//   - its test files, which [go.dokimi.dev/techne/lang.JavaScriptTest]
//     names
//   - its tree-sitter grammar and tags query
//   - typescript-language-server, its language server
//
// A composition root calls [Register], which adds JavaScript to a registry
// and its engines to a catalogue.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/source, lang,
// lang/engines, lang/lsp, lang/treesitter and the tree-sitter grammar of
// JavaScript. It does not import another language module.
package javascript
