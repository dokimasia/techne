// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "go.dokimi.dev/techne/core/source"

// Symbol is one declaration.
type Symbol struct {
	ID       ID
	Name     string
	Kind     Kind
	Language source.Language
	Span     source.Span
	// Parent is the declaration this one sits inside, empty at the top
	// level of a unit.
	Parent ID
	// Exported reports whether the name is visible outside its unit.
	// What that means is the language's decision, not this package's.
	Exported bool
	// Doc is the documentation comment. The output budget drops this
	// before it drops anything else, so a caller must not depend on it.
	Doc string
}

// Unit is what a language calls the thing a file belongs to: a package
// in Go, a module in Python, a crate module in Rust.
type Unit struct {
	ID       ID
	Name     string
	Language source.Language
	Root     source.Path
	Files    []source.Path
}
