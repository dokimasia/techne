// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether a path holds tests, following the sbt layout
// and the ScalaTest naming convention.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "Spec.scala") ||
		strings.HasSuffix(base, "Test.scala") ||
		strings.Contains(p, "src/test/scala/")
}

// Visibility is always unknown at this tier.
//
// Scala spells visibility as a private or protected modifier, which a
// name carries nothing of.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}

// Namespace maps a file path to the name an import would use for it,
// dropping the extension.
func Namespace(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}
