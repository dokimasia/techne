// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/rust"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of Rust declaration. The trait Readable and its implementation for
// Store both declare the method get.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: rust.Declaration(),
		Grammar:     rust.Grammar(),
		Server:      rust.Server(),
		Register:    rust.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.rs": `use std::io::Read;

pub const LIMIT: i32 = 10;
pub static REGISTRY: i32 = 0;

// A note to the next reader, which rustdoc does not read.
pub type Id = String;

pub union Bits {
    raw: i32,
}

/// Store holds items by name.
#[derive(Debug)]
pub struct Store {
    size: i32,
}

pub enum Colour {
    Red,
    Green,
}

pub trait Readable {
    fn get(&self) -> i32;
}

impl Store {
    pub fn new(start: i32) -> Self {
        let value = start;
        Store { size: value }
    }
}

impl Readable for Store {
    fn get(&self) -> i32 {
        self.size
    }
}

/**
 * Returns its argument.
 */
pub fn helper<T>(v: T) -> Option<Box<T>> {
    Some(Box::new(v))
}

pub mod inner {
    pub const NESTED: i32 = 1;
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "Bits", Kind: sema.KindUnion, Signature: "pub union Bits"},
			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "Green", Kind: sema.KindEnumMember},
			{Name: "Id", Kind: sema.KindType},
			{Name: "LIMIT", Kind: sema.KindConstant, Modifiers: []string{"pub", "const"}},
			{Name: "NESTED", Kind: sema.KindConstant},
			{Name: "REGISTRY", Kind: sema.KindVariable},
			{Name: "Read", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "Read"},
			{Name: "Readable", Kind: sema.KindInterface},
			{Name: "Red", Kind: sema.KindEnumMember},
			{
				Name: "Store", Kind: sema.KindImplementation,
				Signature: "impl Store",
			},
			{
				Name: "Store", Kind: sema.KindImplementation,
				Signature: "impl Readable for Store",
			},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:         "Store holds items by name.",
				Annotations: []conformance.Annotated{{Name: "derive", Text: "#[derive(Debug)]"}},
				Modifiers:   []string{"pub"},
			},
			{Name: "T", Kind: sema.KindTypeParameter, Visibility: sema.Unexported},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "get", Kind: sema.KindMethod},
			{
				Name: "helper", Kind: sema.KindFunction,
				Doc: "Returns its argument.",
			},
			{Name: "inner", Kind: sema.KindModule},
			{Name: "new", Kind: sema.KindMethod},
			{Name: "raw", Kind: sema.KindField},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "v", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "value", Kind: sema.KindVariable, Visibility: sema.Unexported},
		},
	})
}
