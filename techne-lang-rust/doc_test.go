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
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: rust.Declaration(),
		Grammar:     rust.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"src/service.rs": `pub mod store {
    pub trait Readable {
        fn get(&self, id: &str) -> i32;
    }

    pub struct Store {
        n: i32,
    }

    impl Store {
        pub fn new() -> Self { Store { n: 1 } }
    }

    fn helper() -> i32 { 1 }
}
`,
		},
		// Rust spells visibility with pub, a modifier the tags query
		// does not capture, so visibility is unknown throughout.
		Declares: []conformance.Declared{
			{Name: "store", Kind: sema.KindModule, Visibility: sema.VisibilityUnknown},
			{Name: "Readable", Kind: sema.KindInterface, Visibility: sema.VisibilityUnknown},
			{Name: "Store", Kind: sema.KindType, Visibility: sema.VisibilityUnknown},
			{Name: "helper", Kind: sema.KindFunction, Visibility: sema.VisibilityUnknown},
		},
	})
}
