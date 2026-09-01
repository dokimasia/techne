// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// Namespace maps a file path to the name an import would use for it,
// dropping the extension: a/b/c becomes the module path a/b/c.
func Namespace(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}

// IsTest reports whether a path holds tests, following the Maven and
// Gradle layout and the JUnit naming convention.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "Test.java") ||
		strings.HasSuffix(base, "Tests.java") ||
		strings.Contains(p, "src/test/java/")
}

// Visibility is always unknown at this tier.
//
// Java spells visibility as a modifier on the declaration, and a tags
// query captures the name rather than the modifiers. Reading the name
// would report every declaration as public, which is the claim this tier
// cannot support.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}
