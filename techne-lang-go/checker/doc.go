// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package checker answers about Go by type-checking the workspace in
// this process.
//
// # Why it exists beside a language server
//
// gopls answers the same questions, and answers them faster once it is
// warm. It is also a program that has to be installed. A machine without
// it drops Go from every question that needs a type — what implements
// this, what calls this, does this still compile — and Go is the one
// language whose toolchain is already there, because it is what the
// workspace is built with.
//
// So this is the floor rather than the ceiling. Where both can answer
// the catalogue prefers the server: both claim [trust.Resolved], both
// cost a session, and the order between equals is the order they were
// registered.
//
// # What it answers
//
// Four roles, each out of one type-checked reading of the module:
//
//   - [Engine.Resolve] over what the checker bound the name at a
//     position to
//   - [Engine.Relate] over uses, calls, implementations and embedding
//   - [Engine.Verify] over what the checker objected to
//   - [Engine.Check] over the same, with the change overlaid on the
//     workspace so a gate judges what has not been written
//
// Outlining and searching are a parser's, at a thousandth of the cost.
// Imports are a parser's too: they are written in the source rather than
// resolved from it. Planning a change is nobody's here — the operations
// techne serves are the ones a server computes, and a rename worked out
// from a type graph by hand would be a second implementation of the one
// thing the write path must not get wrong.
//
// # It is measured against gopls
//
// Not against a list written from memory. Over one module, sixteen
// questions — who refers to this, what implements it, who calls it —
// answered name for name, file for file and line for line the same as
// gopls did. The two differences that comparison found were both this
// package naming the wrong enclosing declaration for a use, and both are
// fixed.
//
// # One reading, kept until the workspace moves
//
// Type-checking a module is seconds. A caller asking who calls a
// function and then who calls its caller would pay it twice, so the
// reading is kept and re-made when a stamp over every Go file in the
// workspace no longer matches. That stamp is what notices a file added
// or taken away, which the loaded file list alone would not.
//
// # Dependency position
//
// Imports core, lang and golang.org/x/tools/go/packages. The loader is
// taken as a dependency for module and build-tag resolution: which files
// belong to a package under which constraints is the go command's
// answer, and a second implementation of it would disagree with the
// compiler exactly where the answer matters.
package checker
