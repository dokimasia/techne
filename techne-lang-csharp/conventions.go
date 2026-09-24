// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp

import (
	"path"
	"strings"
)

// IsTest reports whether p is a test file by the names of xUnit, NUnit and
// MSTest projects: a base name that ends in Test.cs or Tests.cs, and a file
// inside a project directory whose name ends in .Tests.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "Test.cs") ||
		strings.HasSuffix(base, "Tests.cs") ||
		strings.Contains(p, ".Tests/")
}
