// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration of a use", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "use.go"},
				at(t, use, "held := Store"))
			assert.NoError(t, err, "Resolve of a use of Store")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declaration of the use")
			assert.Equal(t, got.Items[0].Kind, sema.KindStruct, "the kind of Store")
			assert.Equal(t, got.Items[0].Span.Path, source.Path("store.go"), "the file of Store")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns the source line of the declaration as its snippet", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "use.go"},
				at(t, use, "held := Store"))
			assert.NoError(t, err, "Resolve of a use of Store")
			assert.Equal(t, got.Items[0].Snippet, "type Store struct {", "the snippet of Store")
		})

		t.Run("returns the declaration at a line and a column", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "use.go"},
				source.Position{Line: 3, Column: 10})
			assert.NoError(t, err, "Resolve at line 3, column 10")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declaration at the line and the column")
		})

		t.Run("returns a declaration at its own name", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "store.go"},
				at(t, store, "func helper"))
			assert.NoError(t, err, "Resolve of the name of helper")
			assert.Equal(t, names(got.Items), []string{"helper"}, "the declaration at its name")
		})

		t.Run("returns no declaration at a keyword", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "use.go"}, source.Position{})
			assert.NoError(t, err, "Resolve at the keyword package")
			assert.Empty(t, got.Items, "the declarations at the keyword")
		})

		t.Run("returns a skipped result for a scope without a Go file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "go.mod"}, source.Position{})
			assert.NoError(t, err, "Resolve in go.mod")
			assert.True(t, got.Skipped, "the answer is skipped")
		})

		t.Run("declines a directory with Go files", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "."}, source.Position{})
			assert.ErrorIs(t, err, engine.ErrDecline, "Resolve in the root directory")
		})

		t.Run("declines a file that no package compiles", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["excluded.go"] = "//go:build ignore\n\npackage p\n\nvar Held = Store{}\n"
			_, err := serving(t, files).Resolve(t.Context(), engine.Request{Scope: "excluded.go"},
				source.Position{Offset: 40})
			assert.ErrorIs(t, err, engine.ErrDecline, "Resolve in a file that the build excludes")
			assert.Contains(t, err.Error(), "excluded.go", "the file in the reason")
		})
	})
}
