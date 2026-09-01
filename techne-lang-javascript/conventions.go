// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether a path holds tests, following the convention
// Jest, Vitest and Mocha discovery use.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasPrefix(p, "__tests__/") ||
		strings.Contains(p, "/__tests__/")
}

// Visibility is always unknown at this tier.
//
// JavaScript marks what leaves a module with an export keyword on the
// declaration, which a name carries nothing of.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}

// Namespace maps a file path to the name an import would use for it,
// dropping the extension.
func Namespace(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}
