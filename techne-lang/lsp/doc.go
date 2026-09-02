// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lsp answers about a language by asking its language server.
//
// # One server, kept warm
//
// A server is started on the first question and kept until the engine is
// closed. The measurements that decide this, taken against gopls on this
// repository: initialise answers in 26ms, the first document symbol in
// 90ms, and every one after that in one or two. Started per call, every
// question would pay the first one's price; kept, a server answers as
// fast as the parser does and knows what the parser cannot.
//
// A caller that asks nothing starts nothing, so a binary serving ten
// languages does not start ten processes to answer about one.
//
// # What it answers
//
// Seven roles, each over the request the specification defines for it:
//
//   - [Engine.Outline] over textDocument/documentSymbol
//   - [Engine.Search] over workspace/symbol
//   - [Engine.Resolve] over textDocument/definition
//   - [Engine.Relate] over textDocument/references, the call hierarchy,
//     textDocument/implementation and the type hierarchy, one per
//     direction
//   - [Engine.Plan] over textDocument/rename for renaming a declaration,
//     workspace/willRenameFiles for moving a file, and a code action for
//     lifting a run of lines into a function
//   - [Engine.Format] over textDocument/formatting, which is the
//     language's own formatter rather than techne's opinion of it
//   - [Engine.Verify] over textDocument/diagnostic, or over what a
//     server publishes unasked when it has no such request
//
// A question with no request behind it declines rather than answering
// none. None is a claim that there are none, and a language that has
// imports would be reported as having no imports rather than as not
// having been asked.
//
// # Nothing here writes
//
// [Engine.Plan] returns the edits and touches no file. techne's own
// write path reads, gates and applies them, which is why a server
// offering an edit unprompted is refused: an edit arriving that way is
// previewed by nothing and checked by nothing. An edit techne asked a
// server to compute is kept and becomes the plan, and goes through the
// same gate.
//
// # A server answers about the buffers it holds
//
// Not about the files. Every question re-sends what a file now holds
// where it differs from what the server was given, and every question
// refreshes all of them rather than the one it names: techne's own write
// path rewrites files underneath the server, and a rename asks who uses
// a declaration, which is answered out of files the question never
// named.
//
// # A tier per role, declared by the language
//
// A server binds names through a type checker, and outlines one file no
// better than a parser does at a thousandth of the speed. [Server.Serves]
// therefore names a tier per role rather than one for the engine: a
// server claiming resolved for outline would win the catalogue's sort
// and start a process to do worse.
//
// # Declared whether or not it is installed
//
// Every language declares its server. One that is not on this machine
// reports why through [Engine.Available], and a capability report says
// so. A missing tool is a problem a caller can act on; a missing
// capability is one to route around, and being told nothing makes them
// look the same.
//
// # What it counts in
//
// The protocol counts lines and UTF-16 code units, and techne counts
// bytes. Every position crossing this boundary is converted, because a
// line holding an emoji moves every span after it and an edit computed
// from an unconverted one writes over half a character.
//
// # Dependency position
//
// Imports core, lang, and the protocol bindings at go.lsp.dev. The
// bindings are taken as a dependency for their union types rather than
// for framing: a definition is a location, a list of locations or a list
// of links, and a document symbol answer is a tree or a flat list.
// Decoding the wrong arm of one of those by hand does not fail — it
// yields entries with every field empty, which reads as a file that
// declares nothing.
//
// What the bindings do not do is convert coordinates. They model the
// protocol's own faithfully, so [Engine] converts.
package lsp
