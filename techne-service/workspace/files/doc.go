// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package files opens the directory of a workspace for the engines, which read it, and for
// the write path, which changes it. [Root] serves both from one [os.Root], so a plan is
// written back to the directory that it was computed from.
//
// # Paths inside the directory
//
// [os.Root] refuses a path that leaves the directory, through .. or through a symbolic link.
// [Root.Write] follows a link inside the directory and changes the file that it links to.
//
// # Writes
//
// [Root.Write] stages the content beside the file, with the suffix [Partial], and renames it
// over the file. A rename in one directory is atomic, so a process that stops leaves the
// original file. A file keeps its mode, and a new file gets 0644 less the umask of the
// process. [Root.Move] renames a file with its mode.
//
// # The lock of a workspace
//
// [Root.Lock] takes an advisory lock of the operating system, which one writer of the
// workspace takes at a time in every techne process: flock(2) on Linux, macOS and the BSDs,
// and LockFileEx on Windows. The lock file is under the user cache directory, outside the
// workspace, and the operating system releases the lock of a process that exits.
//
// # Dependency position
//
// Imports the standard library, core/source, and golang.org/x/sys/windows on Windows. It
// implements the Files port of the write path without importing it, because a port is
// declared where it is consumed.
package files
