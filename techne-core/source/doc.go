// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package source names where code lives: the language a file is written
// in, the path it sits at, and the bytes it occupies.
//
// # Coordinates
//
// Offsets, lines and columns are zero-based and counted in bytes.
// [Position.Offset] is authoritative; [Position.Line] and
// [Position.Column] travel with it so an answer stays readable without
// the file to hand. An engine speaking a protocol that counts UTF-16
// code units converts at its own boundary and never hands those units
// across: a position right in one unit and wrong in the other falls
// inside a token and usually still compiles.
//
// # Paths
//
// [Path] is slash-separated and relative to the workspace root on every
// platform. Absolute paths do not cross a port, because they leak the
// machine's directory layout and make an answer useless elsewhere.
//
// # Ranges
//
// [Span] is half-open: Start is included, End is not. The zero Span
// names no file and covers nothing.
//
// # Dependency position
//
// Imports the standard library only. Every other package in core sits on
// this one.
package source
