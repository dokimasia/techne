// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package c holds everything true of C and nothing true of
// any other language: its declaration, its tags query, and the engines
// only it can use.
//
// # What this module is
//
// [Declaration] states the facts about C that hold whichever
// engine serves it. [Grammar] pairs the compiled grammar with the tags
// query, which is upstream's followed by this module's own patterns.
// [Register] puts both into a registry and a catalogue.
//
// # Dependency position
//
// Imports core, lang and this language's tree-sitter grammar, and never
// another language module. Deleting this directory and its line in
// go.work removes C support completely.
package c
