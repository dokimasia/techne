// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether a path holds tests, following the layout
// RSpec and Minitest both use.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "_test.rb") ||
		strings.HasSuffix(base, "_spec.rb") ||
		strings.HasPrefix(p, "spec/") ||
		strings.Contains(p, "/spec/")
}

// Visibility is always unknown at this tier.
//
// Ruby sets visibility with a private or protected call that applies to
// everything declared after it. A tags query captures a declaration
// rather than the statements before it, so a name carries nothing of the
// rule.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}

// Namespace maps a file path to the name an import would use for it,
// dropping the extension.
func Namespace(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}
