// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lsptest runs a scripted language server for the tests of package lsp.
//
// The server is the test binary itself. [Server] returns a declaration whose command is the
// running test binary and whose environment selects a [Mode]. [Main], called from TestMain,
// runs the script when that environment is present and runs the tests otherwise. No language
// server has to be installed, so the tests of package lsp return the same result on every
// machine.
//
// # Modes
//
// Each [Mode] reproduces one behaviour of a real server. Examples are a definition sent as one
// location, a rename refused as a conflict, and diagnostics published when analysis finishes.
// [Default] responds to every request that package lsp sends with a fixed result about
// [Content].
//
// # Options
//
// An [Option] changes one answer of any mode:
//
//   - [Renames] replaces the answer to textDocument/rename with a workspace edit that the test
//     writes out.
//   - [Outside] makes references, renames and outgoing calls return a file outside the
//     workspace.
//   - [RecordStarts] appends a line to a file each time the server starts.
//   - [RecordRequests] appends the method of each request to a file.
//
// # Fixtures
//
// [Content] and [Emoji] are the files that the ranges of the script point into. [Bundle]
// returns a file of any number of functions on one line, which the Minified mode describes
// from its buffer. [Twins] declares a method Get in each of two types, which the Receivers
// mode reports at the top level and the Impls mode inside impl blocks. A test writes them into
// a workspace with [Workspace].
// [Declaration] declares [Language], which claims files with the [Extension] suffix.
//
// # Engines
//
// [Engine] returns an engine that reads every declaration from the scripted server. [Parsing]
// returns an engine that reads the declarations of a file through [Parser], the outline engine
// of [Language], as a language module passes its tree-sitter engine.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/sema, core/source, core/trust, lang,
// lang/lsp and go.lsp.dev/uri.
package lsptest
