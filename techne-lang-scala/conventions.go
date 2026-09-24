// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala

import (
	"path"
	"strings"
)

// IsTest reports whether p is a test file: a file of the src/test/scala
// tree of sbt, and a base name that ends in Spec.scala or Test.scala, as
// ScalaTest and JUnit name a test class.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "Spec.scala") ||
		strings.HasSuffix(base, "Test.scala") ||
		strings.Contains(p, "src/test/scala/")
}
