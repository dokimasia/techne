// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package app composes techne: the ten language modules and the mock languages, the read and
// write services, the tools and the presenter.
//
// # Languages
//
// [Build] registers each language module by an explicit call, so the set of languages is a
// value of this package and not of the imports of the binary. No other package of techne
// imports a language module. Removing a language takes four edits in the root module:
//
//   - its registration in [Build]
//   - its import in this package
//   - its require and its replace in go.mod
//   - its use line in go.work
//
// # Mock languages
//
// [Build] also registers the mock languages of a specification, which [Run] reads from the
// variable TECHNE_MOCK. A mock language serves every role at the tiers of its entry, so a
// caller can drive the tools at tiers that no language module claims. The empty
// specification registers none.
//
// # Command line
//
// [Parse] reads the command line of techne, and [Usage] is its usage. [Run] serves the
// workspace of the command line, at the root that [Root] resolves.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/source, core/trust, lang, the ten language
// modules, lang/mock, presenter, service/change, service/query, service/workspace/files and
// tool. The command techne is the only package that imports it.
package app
