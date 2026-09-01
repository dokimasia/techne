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
//   - The tags query compiles, so a mistake in a module stops startup
//     rather than returning no results at run time.
//   - Outlining the supplied source finds what the module says it
//     declares, and every symbol found is well formed.
//   - The answer states syntactic evidence over a total scope, and
//     carries the caveat that a name matched across files is
//     coincidence.
//   - Two identical requests answer identically.
//
// # What it does not check
//
// It does not check that a grammar finds every declaration a language
// can express. The tags queries are vendored from upstream and capture
// what upstream chose to capture; a module states what it expects and
// the suite holds it to that.
//
// # Dependency position
//
// Imports core, lang, lang/treesitter, assert and testing. Only a test
// imports this package.
package conformance
