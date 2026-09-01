// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/rust"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form Rust has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: rust.Declaration(),
		Grammar:     rust.Grammar(),
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

/**
 * Returns its argument.
 */
pub fn helper<T>(v: T) -> T {
    v
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
			{Name: "Read", Kind: sema.KindImport},
			{Name: "Readable", Kind: sema.KindInterface},
			{Name: "Red", Kind: sema.KindEnumMember},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:         "Store holds items by name.",
				Annotations: []conformance.Annotated{{Name: "derive", Text: "#[derive(Debug)]"}},
				Modifiers:   []string{"pub"},
			},
			{Name: "T", Kind: sema.KindTypeParameter},
			{Name: "get", Kind: sema.KindMethod},
			{
				Name: "helper", Kind: sema.KindFunction,
				Doc: "Returns its argument.",
			},
			{Name: "inner", Kind: sema.KindModule},
			{Name: "new", Kind: sema.KindMethod},
			{Name: "raw", Kind: sema.KindField},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindParameter},
			{Name: "v", Kind: sema.KindParameter},
			{Name: "value", Kind: sema.KindVariable},
		},
	})
}
