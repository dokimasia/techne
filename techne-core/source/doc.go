// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package source defines where code is: the language of a file, its path,
// and the positions and ranges in its bytes.
//
// # Coordinates
//
// Offsets, lines and columns are zero-based byte counts. [Position.Offset]
// is authoritative. [Position.Line] and [Position.Column] are derived from
// it, so an answer is readable without the file. An engine whose protocol
// counts UTF-16 code units converts them to bytes before it returns a
// position.
//
// # Paths
//
// A [Path] is slash-separated and relative to the workspace root on every
// platform. No port accepts or returns an absolute path.
//
// # Ranges
//
// A [Span] is half-open: it includes Start and excludes End.
//
// # Dependency position
//
// It does not import any package. The packages sema, trust, diag, edit and
// engine of core import it.
package source
