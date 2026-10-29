// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

// Package version returns the version of a build of techne.
//
// The go command writes the build information of a binary. A build at the tag of a release records
// the version of the main module, and a build of a git checkout records the commit and its time.
// [Full] returns the version string of the running binary, and [Of] the version string of any build
// information. [Format] writes a version string, such as 1.2.3 (abc123, built 2026-01-01).
//
// # Releases
//
// The version of a version string is the version of a release without its v, or [Development] for
// a build that is not at the tag of a release:
//
//   - a build without build information
//   - a build that records (devel), such as a build of the workspace
//   - a build at a commit after the last tag, which records a pseudo-version
//   - a build of a working tree with changes
//
// # Dependency position
//
// Imports the standard library, golang.org/x/mod/module and golang.org/x/mod/semver.
package version
