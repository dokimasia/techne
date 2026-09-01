// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript

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

// IsTest reports whether a path holds tests, following the convention
// Jest and Vitest discovery both use.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasPrefix(p, "__tests__/") ||
		strings.Contains(p, "/__tests__/")
}

// Visibility is always unknown at this tier.
//
// TypeScript spells visibility with an export keyword on the
// declaration, and a tags query captures the name rather than the
// keyword. A name carries nothing of it.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}
