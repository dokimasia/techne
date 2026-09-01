// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import "go.dokimi.dev/techne/core/source"

// Symbol is one declaration.
type Symbol struct {
	ID       ID              `json:"id"`
	Name     string          `json:"name"`
	Kind     Kind            `json:"kind"`
	Language source.Language `json:"language"`
	Span     source.Span     `json:"span"`
	// Parent is the declaration this one sits inside, empty at the top
	// level of a unit.
	Parent ID `json:"parent,omitempty"`
	// Visibility is whether the declaration can be named outside its
	// unit, and is [VisibilityUnknown] where the engine could not tell.
	// What visibility means is the language's decision, not this
	// package's.
	//
	// It carries no omitempty: [VisibilityUnknown] is an answer, and a
	// caller that could not tell it from an absent field would read
	// "the engine was unsure" as "the engine said nothing".
	Visibility Visibility `json:"visibility"`
	// Doc is the documentation comment. The output budget drops this
	// before it drops anything else, so a caller must not depend on it.
	Doc string `json:"doc,omitempty"`
	// Snippet is the declaration's own source text, so a caller that
	// found what it wanted needs no second call to read it. The output
	// budget drops it after the documentation and before it drops any
	// item, so a caller must not depend on it.
	Snippet string `json:"snippet,omitempty"`
}

// Unit is what a language calls the thing a file belongs to: a package
// in Go, a module in Python, a crate module in Rust.
type Unit struct {
	ID       ID              `json:"id"`
	Name     string          `json:"name"`
	Language source.Language `json:"language"`
	Root     source.Path     `json:"root"`
	Files    []source.Path   `json:"files,omitempty"`
}
