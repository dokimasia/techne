// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package version returns the version of a build of techne.
//
// The release build sets three variables with -X flags of the linker, and [Full] writes them
// with [Format]:
//
//	go.dokimi.dev/techne/internal/version.buildVersion  the version of the release
//	go.dokimi.dev/techne/internal/version.buildCommit   the commit of the release
//	go.dokimi.dev/techne/internal/version.buildDate     the date of the commit
//
// A build without the flags has the version dev.
//
// # Dependency position
//
// The package does not import another package.
package version
