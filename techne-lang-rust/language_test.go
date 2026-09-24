// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/rust"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language rust", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Declaration().Language, rust.Language, "the language of the declaration")
			assert.Equal(t, string(rust.Language), "rust", "the value of Language")
		})

		t.Run("claims the extension of Rust", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Declaration().Extensions, []string{".rs"}, "the extensions of Rust")
		})

		t.Run("lists the manifests of a Rust project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Declaration().Manifests, []string{"Cargo.toml", "rust-project.json"},
				"the manifests of Rust")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Declaration().Namespace("src/store.rs"), "src/store", "the unit of src/store.rs")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "helper"} {
				assert.Equal(t, rust.Declaration().Visibility(name), sema.VisibilityUnknown, "the visibility of "+name)
			}
		})

		t.Run("ignores the wildcard pattern", func(t *testing.T) {
			t.Parallel()
			assert.True(t, rust.Declaration().Blank["_"], "the blank identifiers of Rust")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs rust-analyzer without arguments", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Server().Command, []string{"rust-analyzer"}, "the command of rust-analyzer")
		})

		t.Run("opens a file as rust", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Server().LanguageID, lsp.IdentityRust, "the language identifier of rust-analyzer")
		})

		t.Run("extracts a function by the title of its code action", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Server().Extracts,
				lsp.Refactor{Kind: "refactor.extract", Titles: []string{"into function"}},
				"the extraction of rust-analyzer")
		})

		t.Run("names the checks that its diagnostics leave out", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rust.Server().Unchecked, "lifetimes or borrows", "the unchecked of rust-analyzer")
		})
	})
}
