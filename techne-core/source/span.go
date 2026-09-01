// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source

// Path locates a file, slash-separated and relative to the workspace
// root on every platform. The zero Path is empty and names no file.
type Path string

// Position is one point in a file: a byte offset, and the line and
// column it falls on. All three are zero-based and counted in bytes.
//
// The zero Position is the first byte of a file.
type Position struct {
	// Offset is the authoritative coordinate. Line and Column are
	// derived from it and are carried so an answer reads without the
	// file.
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Span is a half-open range over one file: Start is included, End is
// not. An empty range has Start equal to End and is how an insertion
// point is expressed.
//
// The zero Span names no file and covers nothing.
type Span struct {
	Path  Path     `json:"path"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}
