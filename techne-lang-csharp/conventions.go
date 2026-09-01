// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether a path holds tests, following the naming
// xUnit, NUnit and MSTest projects use.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "Test.cs") ||
		strings.HasSuffix(base, "Tests.cs") ||
		strings.Contains(p, ".Tests/")
}

// Visibility is always unknown at this tier.
//
// C# spells visibility as a modifier on the declaration. A tags query
// captures the name rather than the modifiers, and reading the name
// would report every declaration as public.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}

// Namespace maps a file path to the name an import would use for it,
// dropping the extension.
func Namespace(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}
