// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lsp serves the questions of one language through its language server.
//
// [Engine] runs one server over LSP 3.17 on stdin and stdout. The server starts on the first
// question and runs until [Engine.Close], so a binary that serves ten languages starts only
// the servers it is asked about. A language module declares its server with a [Server], and
// the engine serves the language beside the tree-sitter engine.
//
// # Roles
//
// Each role sends the request of LSP 3.17 for it:
//
//   - [Engine.Resolve]: textDocument/definition.
//   - [Engine.Relate]: textDocument/references, the call hierarchy,
//     textDocument/implementation and the type hierarchy.
//   - [Engine.Plan]: textDocument/rename to rename a declaration, workspace/willRenameFiles to
//     move a file, and a code action to extract a function.
//   - [Engine.Format]: textDocument/formatting.
//   - [Engine.Check]: the diagnostics of content that the server receives as unsaved buffers.
//   - [Engine.Verify]: textDocument/diagnostic, or the diagnostics a server publishes.
//
// A role returns [engine.ErrDecline] for a request that the server did not offer at
// initialize, so the catalogue passes the request to the next engine. An engine claims the
// tier that its [Server] declares for each role. [Binding] returns the tiers of a server with
// a type checker.
//
// # Identities
//
// [sema.Qualify] builds the qualified name in the ID of a symbol from the container that the
// server reports:
//
//   - the symbol that contains it in a DocumentSymbol tree
//   - the containerName of a SymbolInformation entry
//   - the qualifier in the reported name of a symbol at the top level: Store for the gopls
//     method (*Store).Get
//
// A symbol is no container when the parser does not declare one for it, and its children take
// its place:
//
//   - a symbol whose kind declares nothing
//   - a symbol of the kind File, which csharp-ls reports around the declarations of a file
//   - an anonymous function or class, to which the TypeScript server gives the name <function>
//     or <class>, or the name of the call it is passed to
//
// The Object that rust-analyzer reports for an impl block is an implementation, with the name
// that the parser gives the block: Store for impl Getter for Store<T>.
//
// An import is not qualified. The engine maps an ID to the symbol with that ID. Without one,
// it tries these steps in order, and takes the symbol of a step that selects exactly one:
//
//   - the symbol with the qualified name of the ID, of any kind
//   - the symbol of the kind of the ID whose qualified name and the qualified name of the ID
//     end in one another at a dot, as Shop.Store and Store do
//   - the symbol whose name is the base of the qualified name of the ID
//
// A parser and a server can classify or nest one declaration differently, and the steps map
// the ID of the parser to the symbol of the server. A step that selects two symbols ends the
// search without a symbol.
//
// # Evidence
//
// A question waits for the server to settle: no open work-done progress job, and no activity
// for 300 milliseconds, for at most [Server.Loading]. An answer from a server that has not
// settled is partial. An empty answer from a server whose diagnostics do not show that it
// analysed the file is partial.
//
// The errors that the server reports for the project of an answer lower the answer by the rule
// of [lang.Lowered]. Only an error on a line that can hide a use of the name that the answer is
// about lowers it to [trust.Indexed]. An extraction rewrites no reference and is never lowered.
//
// # Buffers
//
// A server analyses its buffers, not the files on disk. Before each question the engine sends
// every buffer whose file changed on disk since the server received it, and releases every
// buffer whose file is gone. It compares the size and the modification time of each file, and
// reads a file only when one of them changed.
//
// # Writes
//
// [Engine.Plan] returns changes and does not apply them. The write path of techne seals,
// gates and applies them. A server that sends workspace/applyEdit receives a reply that
// nothing was applied.
//
// # Positions
//
// The protocol counts a line and a character in UTF-16 code units, and techne counts bytes.
// The engine converts every position in both directions.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/engine, core/sema, core/source,
// core/trust, lang, and the protocol binding at go.lsp.dev: protocol, jsonrpc2 and uri. The
// binding decodes the union types of the protocol, such as a definition that is one location,
// a list of locations or a list of links.
package lsp
