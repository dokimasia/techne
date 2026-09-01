// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import (
	"slices"

	"go.dokimi.dev/techne/core/source"
)

// Annotation is metadata written onto a declaration rather than a
// declaration of its own.
//
// It covers what each language calls something different and uses the
// same way: a Java annotation, a Python or TypeScript decorator, a Rust
// attribute, a C# attribute list, a Go struct tag. None of them binds a
// name, so none is a [Symbol]. They reach a caller here because they
// decide what a tool rewriting the declaration must reproduce.
//
// Applying an annotation is this. Declaring the annotation type is a
// [Symbol] of [KindAnnotation].
type Annotation struct {
	// Name is the annotation without its punctuation or arguments:
	// "Injectable" for @Injectable({scope: 1}), "derive" for
	// #[derive(Debug)], "json" for a `json:"id"` struct tag.
	Name string `json:"name"`
	// Text is the whole thing as written, punctuation and arguments
	// included. A tool reproducing an annotation needs what was there,
	// not a reconstruction of it.
	Text string `json:"text"`
	// Span locates the annotation itself, not what it is attached to.
	Span source.Span `json:"span"`
}

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
	// Modifiers are the keywords qualifying the declaration, in source
	// order: static, final, abstract, async, pub, export, readonly. For
	// the languages that spell visibility as a modifier this is the only
	// place it is written.
	Modifiers []string `json:"modifiers,omitempty"`
	// Annotations are the metadata attached to this declaration.
	Annotations []Annotation `json:"annotations,omitempty"`
	// Signature is the declaration without its body: what a caller needs
	// to call it, implement it or match it, and nothing of how it works.
	// A declaration that has no body is its own signature.
	Signature string `json:"signature,omitempty"`
	// Doc is the documentation comment. The output budget drops this
	// before it drops anything else, so a caller must not depend on it.
	Doc string `json:"doc,omitempty"`
	// Snippet is the declaration's own source text, so a caller that
	// found what it wanted needs no second call to read it. The output
	// budget drops it after the documentation and before it drops any
	// item, so a caller must not depend on it.
	Snippet string `json:"snippet,omitempty"`
}

// Annotated reports whether the declaration carries an annotation of
// this name.
func (s Symbol) Annotated(name string) bool {
	for _, a := range s.Annotations {
		if a.Name == name {
			return true
		}
	}
	return false
}

// Modified reports whether the declaration carries this keyword.
func (s Symbol) Modified(keyword string) bool {
	return slices.Contains(s.Modifiers, keyword)
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
