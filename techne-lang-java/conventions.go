// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java

import (
	"path"
	"strings"
)

// IsTest reports whether p is a test file: a file of the src/test/java
// tree of Maven and Gradle, and a base name that ends in Test.java or
// Tests.java, as JUnit names a test class.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "Test.java") ||
		strings.HasSuffix(base, "Tests.java") ||
		strings.Contains(p, "src/test/java/")
}
