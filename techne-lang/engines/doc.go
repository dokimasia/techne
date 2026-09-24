// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package engines builds the engines of one language from the declaration, the grammar and the
// server that its module states.
//
// A language module calls [Register] from its own Register function, so the rule that decides
// the engines of a workspace is in one place for the ten modules.
//
// # Engines of a workspace
//
//   - The tree-sitter engine, over every workspace. It reads through an [io/fs.FS] and needs
//     nothing installed.
//   - The server engine, when the module declares a server and the workspace is on disk. A
//     server opens files by name, so a workspace in memory has no server engine. The server
//     engine reads the declarations of a file through the tree-sitter engine.
//   - The engines that a module passes of its own, such as the type checker of Go.
//
// A server that is not on PATH still has an engine. Its
// [go.dokimi.dev/techne/lang/lsp.Engine.Available] names the program to install.
//
// # Dependency position
//
// Imports the standard library, core/engine, lang, lang/lsp and lang/treesitter.
package engines
