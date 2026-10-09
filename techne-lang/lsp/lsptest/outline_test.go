// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// importing is a file whose line 2 imports other and a/store. The name a/store starts at
// character 14 of the line and ends at character 21.
const importing = "package b\n\nimport other, a/store\n"

// naming is a file whose line 2 imports value from a/store. The module a/store starts at
// character 5 of the line and ends at character 12, and the name value starts at character 20.
const naming = "package b\n\nfrom a/store import value\n"

// store is the ID of a variable store at the root of a workspace, whose imports a test relates.
var store = sema.NewID(lsptest.Language, ".", "store", sema.KindVariable)

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

		kinds := []struct {
			name string
			give string
			want sema.Kind
		}{
			{
				name: "returns an interface for a line that starts with type and contains interface",
				give: "package a\n\ntype Getter interface {\n}\n",
				want: sema.KindInterface,
			},
			{
				name: "returns a type for a line that starts with type and contains no struct or interface",
				give: "package a\n\ntype Getter int\n",
				want: sema.KindType,
			},
		}
		for _, tt := range kinds {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, named(t, outlined(t, tt.give), "Getter").Kind, tt.want, "the kind of Getter")
			})
		}

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
			names := make([]string, 0, len(got))
			for _, one := range got {
				names = append(names, one.Name)
			}
			assert.Equal(t, names, []string{"F0", "F1", "After"}, "the functions of the bundle")
			after := named(t, got, "After")
			assert.HasPrefix(t, lsptest.Bundle(2)[after.Span.Start.Offset:], "func After()",
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

		t.Run("returns each name of a var line with the span of the line", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, lsptest.Pair)
			s, u := named(t, got, "s"), named(t, got, "t")
			assert.Equal(t, s.Kind, sema.KindVariable, "the kind of s")
			assert.Equal(t, u.ID, sema.NewID(lsptest.Language, ".", "t", sema.KindVariable), "the ID of t")
			assert.Equal(t, lsptest.Pair[u.Span.Start.Offset:u.Span.End.Offset], "var s, t = 1, 2",
				"the text of the span of t")
			assert.Equal(t, s.Span, u.Span, "the span of s")
		})

		t.Run("returns a field for a line that starts with field", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, lsptest.Literal)
			field := got[slices.IndexFunc(got, func(one sema.Symbol) bool { return one.Kind == sema.KindField })]
			assert.Equal(t, lsptest.Literal[field.Span.Start.Offset:field.Span.End.Offset], "field size int",
				"the text of the span of the field")
			assert.Equal(t, field.Parent, named(t, got, "Item").ID, "the parent of the field")
		})

		t.Run("returns a field for each shorthand property", func(t *testing.T) {
			t.Parallel()
			var parents []sema.ID
			for _, one := range outlined(t, lsptest.Literal) {
				if one.Kind == sema.KindField && lsptest.Literal[one.Span.Start.Offset:one.Span.End.Offset] == "size" {
					parents = append(parents, one.Parent)
				}
			}
			assert.Equal(t, parents, []sema.ID{
				sema.NewID(lsptest.Language, ".", "Make", sema.KindFunction),
				sema.NewID(lsptest.Language, ".", "Copy", sema.KindFunction),
			}, "the parents of the shorthand properties")
		})

		t.Run("returns an import for each name of an import line with the span of the line", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, importing)
			other, imported := named(t, got, "other"), named(t, got, "a/store")
			assert.Equal(t, imported.Kind, sema.KindImport, "the kind of a/store")
			assert.Equal(t, importing[imported.Span.Start.Offset:imported.Span.End.Offset], "import other, a/store",
				"the text of the span of a/store")
			assert.Equal(t, other.Span, imported.Span, "the span of other")
		})

		t.Run("returns an import of the module of a from line with the span of the line", func(t *testing.T) {
			t.Parallel()
			module := named(t, outlined(t, naming), "a/store")
			assert.Equal(t, module.Kind, sema.KindImport, "the kind of a/store")
			assert.Equal(t, naming[module.Span.Start.Offset:module.Span.End.Offset], "from a/store import value",
				"the text of the span of a/store")
		})

		t.Run("returns an import of the name of a from line with the span of the name", func(t *testing.T) {
			t.Parallel()
			name := named(t, outlined(t, naming), "value")
			assert.Equal(t, name.Kind, sema.KindImport, "the kind of value")
			assert.Equal(t, naming[name.Span.Start.Offset:name.Span.End.Offset], "value",
				"the text of the span of value")
		})

		t.Run("skips a scope without the extension of the language", func(t *testing.T) {
			t.Parallel()
			got, err := lsptest.Parser(t.TempDir()).Outline(t.Context(), engine.Request{Scope: "notes.md"})
			assert.NoError(t, err, "Outline of notes.md")
			assert.True(t, got.Skipped, "the skip of notes.md")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an import line that lists the name", func(t *testing.T) {
			t.Parallel()
			got := importersOf(t, map[string]string{
				"b.fake": "package b\n\nimport store\n", "c.fake": "package c\n\nimport other\n",
			})
			assert.Equal(t, files(got), []string{"b.fake"}, "the files that import store")
		})

		t.Run("returns an import line that lists a name ending in the name after a slash", func(t *testing.T) {
			t.Parallel()
			got := importersOf(t, map[string]string{"a.fake": importing})
			assert.Equal(t, files(got), []string{"a.fake"}, "the files that import store")
		})

		t.Run("returns one relation for a line that lists two names that end in the name", func(t *testing.T) {
			t.Parallel()
			got := importersOf(t, map[string]string{"a.fake": "package b\n\nimport store, a/store\n"})
			assert.Length(t, got, 1, "the relations of the line")
		})

		t.Run("stops at the limit of the request", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{
				"b.fake": "package b\n\nimport store\n", "c.fake": "package c\n\nimport store\n",
			})
			got, err := relator(t, root).Relate(t.Context(), engine.Request{Scope: ".", Limit: 1}, store,
				sema.ImportedBy)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.Equal(t, files(got.Items), []string{"b.fake"}, "the files that import store")
		})

		t.Run("starts a relation at the span of the import line", func(t *testing.T) {
			t.Parallel()
			got := importersOf(t, map[string]string{"a.fake": importing})
			at := got[0].At
			assert.Equal(t, importing[at.Start.Offset:at.End.Offset], "import other, a/store", "the text of the site")
			assert.Equal(t, got[0].Via, "import other, a/store", "the line of the site")
		})

		t.Run("returns the importing file as the far end", func(t *testing.T) {
			t.Parallel()
			got := importersOf(t, map[string]string{"a.fake": importing})
			assert.Equal(t, got[0].To, sema.File(lsptest.Language, "a.fake"), "the far end of the relation")
		})

		t.Run("returns ErrDecline for a kind other than ImportedBy", func(t *testing.T) {
			t.Parallel()
			_, err := relator(t, t.TempDir()).Relate(t.Context(), engine.Request{Scope: "."}, store, sema.Imports)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("returns an error for a file that it cannot read", func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			assert.NoError(t, os.Symlink(filepath.Join(root, "absent"), filepath.Join(root, "gone.fake")),
				"the link gone.fake")
			_, err := relator(t, root).Relate(t.Context(), engine.Request{Scope: "."}, store, sema.ImportedBy)
			assert.HasError(t, err, "Relate over gone.fake")
		})
	})

	t.Run("Shorthands", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the span of each name written alone between braces", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Literal})
			got, err := shorthands(t, root).Shorthands(t.Context(), "a.fake")
			assert.NoError(t, err, "Shorthands of a.fake")
			starts := make([]int, 0, len(got))
			for _, one := range got {
				assert.Equal(t, lsptest.Literal[one.Start.Offset:one.End.Offset], "size", "the text of a span")
				starts = append(starts, one.Start.Offset)
			}
			assert.Equal(t, starts, []int{
				strings.Index(lsptest.Literal, "{ size }") + len("{ "),
				strings.LastIndex(lsptest.Literal, "{ size }") + len("{ "),
			}, "the starts of the shorthand properties")
		})

		t.Run("returns no span for braces around no name", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{
				"a.fake": "package a\n\nfunc Empty() Item { return Item{  } }\n",
			})
			got, err := shorthands(t, root).Shorthands(t.Context(), "a.fake")
			assert.NoError(t, err, "Shorthands of a.fake")
			assert.Empty(t, got, "the shorthand properties of a.fake")
		})

		t.Run("returns no span for a path without the extension of the language", func(t *testing.T) {
			t.Parallel()
			got, err := shorthands(t, t.TempDir()).Shorthands(t.Context(), "notes.md")
			assert.NoError(t, err, "Shorthands of notes.md")
			assert.Empty(t, got, "the shorthand properties of notes.md")
		})

		t.Run("returns an error for a file that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := shorthands(t, t.TempDir()).Shorthands(t.Context(), "gone.fake")
			assert.HasError(t, err, "Shorthands of gone.fake")
		})
	})
}

// shorthands returns the parser of the workspace at root as an engine that reads shorthand
// properties, and fails the test when it is not one.
func shorthands(t *testing.T, root string) lsp.Shorthands {
	t.Helper()
	reads, implements := lsptest.Parser(root).(lsp.Shorthands)
	assert.True(t, implements, "the parser implements lsp.Shorthands")
	return reads
}

// relator returns the parser of the workspace at root as an engine that relates declarations,
// and fails the test when it is not one.
func relator(t *testing.T, root string) engine.Relator {
	t.Helper()
	relates, implements := lsptest.Parser(root).(engine.Relator)
	assert.True(t, implements, "the parser implements engine.Relator")
	return relates
}

// importersOf returns the relations of [sema.ImportedBy] of [store] that the parser returns over a
// workspace of files.
func importersOf(t *testing.T, files map[string]string) []sema.Relation {
	t.Helper()
	got, err := relator(t, lsptest.Workspace(t, files)).Relate(t.Context(), engine.Request{Scope: "."}, store,
		sema.ImportedBy)
	assert.NoError(t, err, "Relate of the importers of store")
	return got.Items
}

// files returns the path of the site of each relation, in order.
func files(relations []sema.Relation) []string {
	out := make([]string, 0, len(relations))
	for _, one := range relations {
		out = append(out, string(one.At.Path))
	}
	return out
}
