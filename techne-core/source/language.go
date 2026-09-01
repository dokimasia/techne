// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source

// Language identifies a language on every request and every answer.
//
// It is a string rather than an integer so it survives a round trip
// through JSON without a lookup table, and so a language registered at
// run time needs no constant declared here. The zero Language is the
// empty string and names nothing.
//
// The constants below are the languages techne ships with. Nothing in
// core may assume the set is closed: the catalogue answers what is
// registered, and a module can register a language absent from this
// list.
type Language string

// The wire form of each language. These strings are embedded in
// [Language] values that reach a caller and, through sema identities,
// an index that outlives the process, so changing one invalidates
// stored data.
const (
	Go         Language = "go"
	Python     Language = "python"
	Java       Language = "java"
	Rust       Language = "rust"
	TypeScript Language = "typescript"
)
