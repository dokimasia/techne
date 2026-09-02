// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package mock is a language that exists to be driven.
//
// # Why a language rather than a test double
//
// Every port core declares can be answered at any tier. Nothing techne
// ships answers above [go.dokimi.dev/techne/core/trust.Syntactic]: a
// parser matches text, so it cannot resolve, relate, or plan a change
// that rewrites references. The tools for those roles would go untested
// against anything but a refusal, and the ports would be shaped by what
// a parser happens to manage rather than by what a caller needs.
//
// This answers all of them, over real files it really reads, and claims
// the tier it really has. Within its own small language it resolves: a
// use names a declaration, and the workspace is read whole, so an empty
// answer means there are none.
//
// # It is a factory, not a language
//
// [Registering] takes a name and returns the registration for a language
// of that name, claiming the extension of the same name. Registering
// three gives a workspace holding three languages that answer
// differently, which is what routing, merging and per-language refusal
// need in order to be exercised at all.
//
// [With] narrows what one of them claims, so a workspace can hold a
// language that resolves beside one that only parses, and the refusals
// between them are real rather than simulated.
//
// # The language
//
// Line-oriented, indentation-nested, and small enough to read in one
// sitting. [Parse] is the whole of it.
//
//	;; Store holds items.
//	type Store
//	  field size
//	func New -> Store
//	  use Store
//
// # Dependency position
//
// Imports core and lang, and nothing else in this repository, as every
// language module does. It compiles without cgo and depends on no
// grammar.
package mock
