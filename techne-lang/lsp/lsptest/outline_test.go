// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// outlined returns the declarations that [lsptest.Parser] reports for the file a.fake with
// content.
func outlined(t *testing.T, content string) []sema.Symbol {
	t.Helper()
	root := lsptest.Workspace(t, map[string]string{"a.fake": content})
	got, err := lsptest.Parser(root).Outline(t.Context(), engine.Request{Scope: "a.fake"})
	assert.NoError(t, err, "Outline of a.fake")
	return got.Items
}

// named returns the declaration of items named name, and fails the test when none is.
func named(t *testing.T, items []sema.Symbol, name string) sema.Symbol {
	t.Helper()
	for _, one := range items {
		if one.Name == name {
			return one
		}
	}
	t.Fatalf("no declaration named %s", name)
	return sema.Symbol{}
}

func TestOutline(t *testing.T) {
	t.Parallel()

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of Content", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, lsptest.Content)
			kinds := map[string]sema.Kind{}
			for _, one := range got {
				kinds[one.Name] = one.Kind
			}
			assert.Equal(t, kinds, map[string]sema.Kind{
				"Store": sema.KindStruct, "Get": sema.KindMethod, "After": sema.KindFunction,
			}, "the declarations of Content")
		})

		t.Run("qualifies a method by the type of its receiver", func(t *testing.T) {
			t.Parallel()
			got := named(t, outlined(t, lsptest.Content), "Get")
			assert.Equal(t, got.ID, sema.NewID(lsptest.Language, ".", "Store.Get", sema.KindMethod), "the ID of Get")
		})

		t.Run("spans a type through its closing brace", func(t *testing.T) {
			t.Parallel()
			got := named(t, outlined(t, lsptest.Content), "Store")
			assert.Equal(t, lsptest.Content[got.Span.Start.Offset:got.Span.End.Offset],
				"type Store struct {\n\tsize int\n}", "the text of the span of Store")
		})

		t.Run("returns each function of a bundle", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, lsptest.Bundle(2))
			var names []string
			for _, one := range got {
				names = append(names, one.Name)
			}
			assert.Equal(t, names, []string{"F0", "F1", "After"}, "the functions of the bundle")
			after := named(t, got, "After")
			assert.True(t, strings.HasPrefix(lsptest.Bundle(2)[after.Span.Start.Offset:], "func After()"),
				"the start of the span of After")
		})

		t.Run("links a variable to the function that contains it", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, "package a\nfunc Use() int {\n\tvar held = 1\n\treturn held\n}\n")
			held := named(t, got, "held")
			assert.Equal(t, held.Kind, sema.KindVariable, "the kind of held")
			assert.Equal(t, held.Parent, named(t, got, "Use").ID, "the parent of held")
			assert.Equal(t, held.ID, sema.NewID(lsptest.Language, ".", "Use.held", sema.KindVariable), "the ID of held")
		})

		t.Run("skips a scope without the extension of the language", func(t *testing.T) {
			t.Parallel()
			got, err := lsptest.Parser(t.TempDir()).Outline(t.Context(), engine.Request{Scope: "notes.md"})
			assert.NoError(t, err, "Outline of notes.md")
			assert.True(t, got.Skipped, "the skip of notes.md")
		})
	})
}
