// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package conformance is the test suite that every language module runs
// against its declaration, its server and its tree-sitter engine.
//
// A language module supplies a [Suite] and calls [Run] from a test. Every
// module runs the same checks, so a change to the shared engine for one
// language fails in every language it breaks.
//
// # Checks
//
//   - Register adds the language and the tree-sitter engine over a tree in
//     memory, and also the server for a workspace on disk. It refuses a
//     second registration of the language.
//   - Every extension routes to the language, and lang.ProjectOf returns
//     the directory of each manifest that the declaration lists.
//   - The server passes lsp.Server.Valid and names the program it runs. It
//     claims trust.Resolved for resolve, and trust.None for outline and
//     search.
//   - The tags query compiles, and New refuses a definition capture without
//     a kind.
//   - The outline of the fixture equals [Suite.Declares] as a set, and
//     every symbol is complete.
//   - Each declaration links to its innermost container, compared with a
//     pairwise scan.
//   - The identity of each member contains the qualified name of its
//     container, a dot and its name. An import is not qualified. A method at
//     the top level of a file may be qualified by its receiver.
//   - Members of one name and kind in different containers of one file have
//     different identities. The fixture declares at least one such pair.
//   - The declarations have the documentation, annotations, signatures and
//     modifiers the fixture states.
//   - The answers are syntactic, with total coverage and the caveat for
//     matched text. A file larger than lang.Largest is reported as unread.
//   - An outline opens the root .gitignore once.
//   - Search ranks an exact match first, and adds a truncation caveat at
//     its limit.
//   - A second search does not read an unchanged file without a match. It
//     reads a file again after its size or its modification time changes.
//   - Index returns the declarations of one file, and skips a file of
//     another language.
//   - Relate skips a scope without a file of the language, and declines a
//     relation that needs name binding.
//   - Relate finds each import by its name and by its simple name. It
//     returns an empty answer for another simple name or another
//     qualifier, one relation for each importing line, and the imports of
//     the file that declares a name.
//   - Documentation that Plan writes parses, and Outline reads it back.
//   - Identical requests return identical answers.
//
// # Limits
//
// The suite compares the engine with a fixture that the author of the query
// also writes, so it detects changes, not gaps that both share. Measuring
// the recall of a query takes a corpus written by others and the compiler
// of the language.
//
// # Dependency position
//
// Imports the standard library, core, lang, lang/lsp, lang/treesitter and
// assert. Only tests import it.
package conformance
