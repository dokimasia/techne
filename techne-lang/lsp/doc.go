// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

// Package lsp serves the questions of one language through its language server.
//
// [Engine] runs one server over LSP 3.17 on stdin and stdout. The server starts on the first
// question and runs until [Engine.Close], so a binary that serves ten languages starts only
// the servers it is asked about. A language module declares its server with a [Server], and
// the engine serves the language beside the tree-sitter engine.
//
// A question waits 10 seconds from the start of a server for its reply to initialize. A server
// that has not replied by then goes on starting for up to 2 minutes, and each question until the
// reply declines with the stderr of the server, so the next engine of the language serves it.
// ruby-lsp installs the gems of its bundle before it replies.
//
// # Roles
//
// Each role sends the request of LSP 3.17 for it:
//
//   - [Engine.Resolve]: textDocument/definition.
//   - [Engine.Relate]: textDocument/references, the call hierarchy,
//     textDocument/implementation and the type hierarchy, and textDocument/definition at the
//     imports of a declaration.
//   - [Engine.Plan]: textDocument/rename to rename a declaration, workspace/willRenameFiles to
//     move a file, and a code action to extract a function.
//   - [Engine.Format]: textDocument/formatting.
//   - [Engine.Check]: the diagnostics of content that the server receives as unsaved buffers.
//   - [Engine.Verify]: textDocument/diagnostic, or the diagnostics a server publishes, and the
//     findings of the check on disk of a server that declares [Server.DiskCheck].
//
// A role returns [engine.ErrDecline] for a request that the server did not offer at
// initialize, so the catalogue passes the request to the next engine. An engine claims the
// tier that its [Server] declares for each role. [Binding] returns the tiers of a server with
// a type checker.
//
// # Relations
//
// A server without the call hierarchy, as typescript-language-server, relates the calls of a
// declaration through the outline engine of the language when that engine implements [Calls]:
//
//   - The callers of a declaration are its references at the name of a call that the engine
//     reads. A reference in a file that the engine cannot read is kept.
//   - The calls of a declaration are the calls that the engine reads inside its span. The far
//     end of each is the declaration at a definition of its callee.
//
// A [Server.Related] server returns, among the references of a declaration, the references of
// the declarations that it redeclares. The engine requests the definition at each reference and
// at each caller, and keeps a site whose definition is the declaration or that has no definition.
//
// A [Server.Imports] server relates the imports of a declaration through the outline engine of
// the language when that engine implements [engine.Relator]. The outline engine finds each
// import that writes the name of the declaration. The engine requests the definition at the end
// of each name of the import, and keeps the import when a definition is in the file of the
// declaration, or under the scope of a request without one. typescript-language-server resolves
// an import specifier to the specifier itself, and the engine then asks at the module of the
// import statement. A server that declares [Server.Resolving] can resolve an import to no file
// while it starts, as metals does while it compiles a build, and the engine asks again at such
// an import once a second until the server has run that long. An import under another name,
// such as an import of a directory, is not found, so the answer is partial.
//
// tsserver searches for the implementations of a declaration in the project of the file of the
// request only. For a [Server.Tsserver] server the engine also asks at a use of the declaration
// in one file of each other project that writes its name, and merges the answers. A
// [Server.Contextual] server returns expressions among the implementations, and the engine keeps
// an implementation at the name of a declaration that can implement, such as a class.
//
// # Scoped servers
//
// A [Server.Scoped] server finds a use only in a file that it has open. Before a rename, a move
// and a relation of the uses of a declaration that other files can use, the engine opens up to
// 200 files of the workspace that write the name. A plan or an answer past them is partial. A
// file writes the name of a declaration on a line that does not start with //. For a
// [Server.Tsserver] server, the engine opens one file of each project, so that tsserver loads
// the project whole. It then opens only the files of no project whose imports lead to the file
// of the declaration or to a loaded project.
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
// analysed the file is partial. A check on disk changes the diagnostics of the files alone, so
// only [Engine.Verify] waits for it. The diagnosis of a server that declares [Server.Diagnosis]
// changes the diagnostics alone as well. [Engine.Verify] and [Engine.Check] send the server a
// request after their buffers, to which the server replies after it has begun the diagnosis of
// each buffer, and then wait for every diagnosis to end.
//
// A question waits 2 seconds for a server without pull diagnostics to publish the diagnostics
// of its files. [Engine.Check] checks a change before the write and waits up to
// [Server.Checking], 30 seconds by default, because a check that ends before the reports arrive
// declines, and a weaker engine then checks the change.
//
// A server whose process has exited is replaced by a new server at the next question. A server
// that returns no symbol of a file whose outline declares symbols has stopped answering, as
// typescript-language-server does after its tsserver exits. The engine stops that server, and
// [Engine.Resolve] and [Engine.Relate] ask a new one once. A new server that has stopped
// answering too declines.
//
// The session records the diagnostics that a server publishes, its progress jobs and its file
// watchers when it reads each message, before a handler of the connection runs. A report is recorded before the
// message after it, such as the end of the job that produced the report or the reply to a later
// request.
//
// The errors that the server reports for the project of an answer lower the answer by the rule
// of [lang.Lowered]. Only an error on a line that can hide a use of the name that the answer is
// about lowers it to [trust.Indexed]. An extraction rewrites no reference and is never lowered.
//
// A rename is partial when the server reports a use that no edit of the plan rewrites. An edit
// rewrites a use when it shares a byte with the use or inserts at one of its ends, because a
// server can report a qualified name as the use and rename its last part, and can rename by the
// least edit. A use whose text does not write the old name, such as the new of a target-typed
// new() in C#, needs no edit. A use in a file that [lang.Readable] refuses is always
// unrewritten, because a plan edits no file that techne does not read. A source that a build
// generates under a directory that .gitignore excludes is one such file.
//
// A rename that moves a file is partial when the server does not serve
// workspace/willRenameFiles, because only that request returns the edits of the paths that name
// the moved file. ruby-lsp moves the file of a class with the class, and leaves the
// require_relative that names the file.
//
// # Buffers
//
// A server analyses its buffers, not the files on disk. Before each question the engine sends
// every buffer whose file changed on disk since the server received it, and releases every
// buffer whose file is gone. It compares the size and the modification time of each file, and
// reads a file only when one of them changed. A server that declares [Server.DiskCheck] also
// receives textDocument/didSave for a file that changed on disk since its last check.
//
// A server reads a file without a buffer from disk, and reads it again when the client reports
// a change of the file. The server can register watchers of such files with
// client/registerCapability, as gopls registers watchers of its Go files and of its go.mod,
// go.sum and go.work files. The session records each registration and walks the workspace for
// the files that the watchers match. Before each question the engine walks the workspace again
// and sends workspace/didChangeWatchedFiles with the creation, the change and the deletion of
// each such file without a buffer, of the kinds that its watchers name. The walk leaves out the
// directories that [lang.Vendored] names and the paths that .gitignore excludes. A walk of the
// 13,445 Go and module files of kubernetes takes 33 to 38 milliseconds warm.
//
// The diagnostics of a buffer are dropped when the engine replaces or releases the buffer. A
// server can publish a report of a file after the release. The next open of the file drops that
// report, because it describes no content that the engine sent.
//
// # Writes
//
// [Engine.Plan] returns changes and does not apply them. The write path of techne seals,
// gates and applies them. A server that sends workspace/applyEdit receives a reply that
// nothing was applied.
//
// A server indents the code of a code action by the indentation that it requests from the
// client. jdtls and typescript-language-server request it under the sections that
// [Server.Indentation] names, and the client returns the indentation of the file on disk, as
// [lang.Indentation] reads it. A server that requests none indents by its own settings.
//
// A rename rewrites every use of a declaration. [TypeScriptSettings] and [NativeSettings] turn
// off the alias that typescript-language-server and tsc --lsp write in an export statement that
// re-exports a renamed declaration. The servers then write a bare new name at a shorthand
// property of an object literal, as blob for { file }. When the outline engine of the language
// implements [Shorthands], the engine writes such an edit out as file: blob or blob: file, from
// the declarations that the definition at the name returns.
//
// # Positions
//
// The protocol counts a line and a character in UTF-16 code units, and techne counts bytes.
// The engine converts every position in both directions.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/engine, core/sema, core/source,
// core/trust, lang, the protocol binding at go.lsp.dev: protocol, jsonrpc2 and uri, and
// github.com/bmatcuk/doublestar/v4. The binding decodes the union types of the protocol, such as
// a definition that is one location, a list of locations or a list of links. doublestar matches
// the glob patterns of the file watchers, whose ** and {a,b} path.Match does not take.
package lsp
