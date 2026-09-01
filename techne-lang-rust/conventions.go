// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust

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

// IsTest reports whether a path holds tests, following Cargo's layout.
// A unit test behind #[cfg(test)] inside a source file is invisible
// here, because the path carries nothing of it.
func IsTest(p string) bool {
	return strings.HasPrefix(p, "tests/") ||
		strings.Contains(p, "/tests/") ||
		path.Base(p) == "tests.rs"
}

// Visibility is always unknown at this tier.
//
// Rust spells visibility with a pub modifier, and a tags query captures
// the name rather than the modifier. A name carries nothing of it, so
// answering would report every declaration as public.
func Visibility(string) sema.Visibility {
	return sema.VisibilityUnknown
}
