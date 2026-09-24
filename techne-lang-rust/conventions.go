// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust

import (
	"path"
	"strings"
)

// IsTest reports whether p is a test file by the layout of Cargo: a file
// inside a tests directory, and a file named tests.rs. A unit test in a
// #[cfg(test)] module of a source file does not make the file a test file.
func IsTest(p string) bool {
	return strings.HasPrefix(p, "tests/") ||
		strings.Contains(p, "/tests/") ||
		path.Base(p) == "tests.rs"
}
