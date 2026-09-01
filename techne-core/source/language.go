// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source

// Language identifies a language on every request and every answer.
//
// The type lives here because core names languages constantly and knows
// none of them. The values do not: each language module declares its
// own, so deleting that module removes every mention of the language
// from the tree.
//
// It is a string rather than an integer so it survives a round trip
// through JSON without a lookup table, and so a language registered at
// run time needs no constant compiled in anywhere.
//
// The set is open. A caller asks the catalogue what is registered rather
// than comparing against a list, and two modules claiming one value are
// rejected at registration rather than colliding silently.
//
// The value is the wire form. It reaches a caller and, through a sema
// identity, an index that outlives the process, so a module changing its
// own value invalidates stored data.
//
// The zero Language is the empty string and names nothing.
type Language string
