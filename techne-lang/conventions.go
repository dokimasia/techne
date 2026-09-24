// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// Stem returns p without its extension, such as a/b/c for a/b/c.py. A
// language whose unit is the file uses Stem as its Namespace.
func Stem(p string) string {
	return strings.TrimSuffix(p, path.Ext(p))
}

// VisibilityByModifier returns sema.VisibilityUnknown for every name. A
// language that declares visibility with a modifier uses it as its
// Visibility, because the name does not show the modifier. The modifiers of
// a declaration are in sema.Symbol.Modifiers.
func VisibilityByModifier(string) sema.Visibility {
	return sema.VisibilityUnknown
}

// JavaScriptTest reports whether p is a test file by the default patterns
// of Jest, Vitest and the Node.js test runner, which include the test
// directory of Mocha. It returns true for these files:
//
//   - a file inside a directory named test or __tests__
//   - a file whose base name without its extension is test or spec
//   - a file whose base name without its extension starts with test-, or
//     ends with .test, .spec, -test or _test
func JavaScriptTest(p string) bool {
	dir, base := path.Split(p)
	for dir != "" {
		var name string
		name, dir, _ = strings.Cut(dir, "/")
		if name == "test" || name == "__tests__" {
			return true
		}
	}
	stem := Stem(base)
	return stem == "test" || stem == "spec" || strings.HasPrefix(stem, "test-") ||
		slices.ContainsFunc(testSuffixes, func(suffix string) bool { return strings.HasSuffix(stem, suffix) })
}

// testSuffixes are the endings of the base name of a JavaScript test file,
// before its extension.
var testSuffixes = []string{".test", ".spec", "-test", "_test"}
