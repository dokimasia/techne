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
// [Session] of the command line, at the root that [Root] resolves.
//
// # Sessions
//
// [Open] returns the tools of a session. A call runs in the workspace of the session unless the
// command line has a trusted folder and the call sets the field
// [go.dokimi.dev/techne/tool.WorkingDirectory] to a directory under it. The call then runs in
// the workspace of that directory, which the session opens at its first call. A symbolic link
// counts at its target, so a link that leads out of a trusted folder is not under it. The session
// keeps the engines of three such workspaces open beside its own. The first call in a fourth
// closes the one that served a call least recently and has no call in flight.
//
// # Dependency position
//
// Imports the standard library, core/engine, core/source, core/trust, lang, the ten language
// modules, lang/mock, presenter, service/change, service/query, service/workspace/files and
// tool. The command techne is the only package that imports it.
package app
