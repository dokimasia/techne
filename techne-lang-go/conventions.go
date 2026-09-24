// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	"go/token"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/sema"
)

// IsTest reports whether p is a test file: a base name that ends in
// _test.go, the rule of the go command.
func IsTest(p string) bool {
	return strings.HasSuffix(path.Base(p), "_test.go")
}

// Visibility returns sema.Exported for a name that starts with an
// upper-case letter, which the Go specification exports from its package,
// and sema.Unexported for any other name. It returns
// sema.VisibilityUnknown for an empty name.
func Visibility(name string) sema.Visibility {
	switch {
	case name == "":
		return sema.VisibilityUnknown
	case token.IsExported(name):
		return sema.Exported
	default:
		return sema.Unexported
	}
}

// Unit returns the directory of p, because a Go package is a directory.
func Unit(p string) string {
	return path.Dir(p)
}
