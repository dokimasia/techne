// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether a path holds tests. C has no toolchain rule
// for this, so it follows the naming the common frameworks discover by.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "_test.c") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasPrefix(p, "tests/") ||
		strings.Contains(p, "/tests/")
}

// Visibility is always unknown at this tier.
//
// C settles what leaves a translation unit with the static keyword and
// with what a header declares. A name carries neither.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}

// Namespace maps a file path to the name an import would use for it,
// dropping the extension.
func Namespace(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}
