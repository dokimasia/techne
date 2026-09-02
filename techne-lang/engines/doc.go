// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package engines builds the engines one language is served by.
//
// A language module states what it is — its declaration, its grammar,
// its language server — and this assembles the engines that follow from
// it. Ten modules would otherwise each carry the same eight lines, and a
// rule in ten places is nine chances for them to disagree about which
// engines a workspace supports.
//
// # What a workspace supports
//
// A parser, always: it reads through an [io/fs.FS] and needs nothing
// installed.
//
// A language server, when the workspace is on disk and the module
// declared one. A server is a process that opens files by name, so a
// tree that was never written to disk has none. A server that is not
// installed is still registered, and reports why through
// [go.dokimi.dev/techne/lang/lsp.Engine.Available] — a language that
// vanished with its server would read as a language techne cannot serve
// at all.
//
// # Dependency position
//
// Imports core, lang, and both engine packages. It is the one place that
// names all of them, which is what keeps a language module from naming
// any.
package engines
