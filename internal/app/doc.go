// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package app is the composition root: the one place that names a
// language.
//
// # Registration is a call
//
// [Build] registers each language explicitly rather than through an init
// with a blank import, so the set techne serves is a value this package
// chose. A test builds a server holding one language, and a smaller
// binary is a different main over the same calls.
//
// # Nothing else knows a language
//
// Deleting a language module and its line in go.work removes it from the
// tree, and this file is the only one that has to change.
//
// # Dependency position
//
// Imports core, presenter and every language module. Nothing imports it.
package app
