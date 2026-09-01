// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package python holds everything true of python and nothing true of
// any other language: its declaration, its vendored tags query, and the
// engines only it can use.
//
// # What this module is
//
// [Declaration] states the facts about python that hold whichever
// engine serves it. [Grammar] pairs the compiled grammar with the tags
// query vendored from upstream. [Register] puts both into a registry and
// a catalogue.
//
// # Dependency position
//
// Imports core, lang and this language's tree-sitter grammar, and never
// another language module. Deleting this directory and its line in
// go.work removes python support completely.
package python
