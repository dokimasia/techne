// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package files is the workspace as a directory that can be written to.
//
// # Reading and writing are one root
//
// An engine reads through an [io/fs.FS], which cannot write. [Root]
// supplies both halves from one open directory, so the bytes a plan was
// computed against and the bytes it is written back over come from the
// same place.
//
// # A path cannot leave the root
//
// [Root] is an [os.Root], so a path that climbs out or follows a symlink
// out is refused by the operating system. A check written here would
// have to be right every time; this one is right because it is not this
// package's to get wrong.
//
// # A write is all or nothing
//
// [Root.Write] stages the content beside the target and renames it over,
// which is atomic on every filesystem techne runs on. A process that
// stops partway leaves the original file rather than half of each, and
// the staged file it leaves behind carries a suffix no language claims.
//
// # Dependency position
//
// Imports the standard library and core/source. It satisfies the write
// path's Files port without importing it: the port is declared where it
// is consumed.
package files
