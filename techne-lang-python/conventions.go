// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python

import (
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether p is a test file by the default patterns of
// pytest: a base name that starts with test_ or ends in _test.py.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
}

// Visibility returns the visibility that PEP 8 gives a name. A leading
// underscore marks a name as internal, so Visibility returns
// sema.Unexported for it. A special name such as __init__ belongs to a
// protocol of the language, so Visibility returns sema.Exported for it, as
// for every other name. It returns sema.VisibilityUnknown for an empty
// name.
func Visibility(name string) sema.Visibility {
	switch {
	case name == "":
		return sema.VisibilityUnknown
	case special(name) || !strings.HasPrefix(name, "_"):
		return sema.Exported
	default:
		return sema.Unexported
	}
}

// special reports whether name has two leading and two trailing
// underscores around at least one other character, as the special names of
// Python have.
func special(name string) bool {
	return len(name) > 4 && strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
}
