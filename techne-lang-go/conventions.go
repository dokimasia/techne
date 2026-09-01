// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	"path"
	"strings"
	"unicode"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether a path holds tests. Go's toolchain decides this
// by filename, so a parser can answer exactly.
func IsTest(p string) bool {
	return strings.HasSuffix(p, "_test.go")
}

// Visibility reads Go's rule: a declaration is visible outside its
// package when its name begins with an upper-case letter. The name
// carries the whole rule, so this is never unknown.
func Visibility(name string) sema.Visibility {
	if name == "" {
		return sema.VisibilityUnknown
	}
	if unicode.IsUpper([]rune(name)[0]) {
		return sema.Exported
	}
	return sema.Unexported
}

// Unit is the directory holding the file, because a Go package is a
// directory.
func Unit(p string) string {
	return path.Dir(p)
}
