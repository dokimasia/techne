// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c

import (
	"path"
	"strings"
)

// IsTest reports whether p is a test file. The C toolchain defines no test
// file, so IsTest applies the common names: a base name that ends in _test.c
// or starts with test_, and a file inside a directory named tests.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "_test.c") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasPrefix(p, "tests/") ||
		strings.Contains(p, "/tests/")
}
