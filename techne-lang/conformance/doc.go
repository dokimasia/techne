// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package conformance is the suite every language module runs.
//
// A language module supplies a [Suite] and calls [Run]. The checks are
// identical for every language, so a module that passes them answers the
// same way as every other module, and a shared engine changed for one
// language cannot quietly break another.
//
// # What it checks
//
//   - The declaration registers, and every extension it claims routes
//     back to it.
//   - The tags query compiles, and naming a capture no kind carries is
//     refused, so a mistake in a module stops startup rather than
//     returning no results at run time.
//   - Outlining the supplied source finds what the module says it
//     declares, and every symbol found is well formed.
//   - The declarations carry the documentation, annotations and
//     keywords the module names, in the forms that language writes
//     them.
//   - The answer states syntactic evidence over a total scope, and
//     carries the caveat that a name matched across files is
//     coincidence.
//   - Two identical requests answer identically.
//
// # Exact, in both directions
//
// [Suite.Declares] is the whole outline, compared as a set: a symbol
// found and not listed fails as surely as one listed and not found. It
// is exact because the alternative proved nothing. Checking only that
// listed symbols appear passes a query that finds a quarter of its
// language, which is what every query here once did. A fixture must
// therefore name every declaration form its language has.
//
// [Declared.Doc] is exact the same way for a module whose fixture
// documents anything: a comment read as documentation that the module
// did not name fails as surely as one it named and the parser did not
// find. That is what holds a language to the forms its own documentation
// tool reads, rather than to every comment above a declaration.
//
// # What it does not check
//
// It cannot say whether a query is right, only that it has not changed.
// The same person writes the query and the fixture, so neither knows
// about the form neither remembered. Establishing that a query finds
// what its language has takes a corpus nobody wrote for this tool and
// that language's own compiler, which is a measurement rather than a
// test.
//
// # Dependency position
//
// Imports core, lang, lang/treesitter, assert and testing. Only a test
// imports this package.
package conformance
