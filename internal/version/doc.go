// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package version reports what this binary was built from.
//
// The values are stamped at link time by the release job. A binary built
// without them reports "dev", so an unstamped build is visible rather
// than claiming a release it is not.
//
// # Dependency position
//
// Imports the standard library only.
package version
