// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package golang holds everything true of Go and nothing true of any
// other language: its declaration, its queries, and the engines only Go
// can use.
//
// The package cannot be named go, because go is a keyword. Its import
// path is go.dokimi.dev/techne/lang/go and callers refer to it as
// golang.
//
// # Dependency position
//
// Imports core and lang, never another language module. Only the root
// module imports this package, so deleting this directory and its line
// in go.work removes Go support completely.
package golang
