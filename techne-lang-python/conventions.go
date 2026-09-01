// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python

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
// pytest and unittest discovery both use.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
}

// Visibility reads Python's convention: a leading underscore marks a
// name as internal. It is a convention rather than a rule the runtime
// enforces, which is why an answer here is a claim about intent.
func Visibility(name string) sema.Visibility {
	if name == "" {
		return sema.VisibilityUnknown
	}
	if strings.HasPrefix(name, "_") {
		return sema.Unexported
	}
	return sema.Exported
}
