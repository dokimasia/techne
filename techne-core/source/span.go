// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source

// Path is a slash-separated file path, relative to the workspace root. The
// zero value is the empty string, which is not a path.
type Path string

// Position is one point in a file. The zero value is the first byte.
type Position struct {
	// Offset is the zero-based byte offset. It is authoritative.
	Offset int `json:"offset"`
	// Line is the zero-based line of Offset.
	Line int `json:"line"`
	// Column is the zero-based byte column of Offset.
	Column int `json:"column"`
}

// Span is a half-open range of one file: it includes Start and excludes End.
// A Span whose Start equals its End is an insertion point. The zero value
// has an empty Path.
type Span struct {
	Path  Path     `json:"path"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}
