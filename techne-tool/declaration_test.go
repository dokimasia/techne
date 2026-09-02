// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/tool"
)

// covering builds one symbol over a byte range of one file, which is
// what decides where it sits in the tree.
func covering(name string, kind sema.Kind, start, end int) sema.Symbol {
	return sema.Symbol{
		Name: name, Kind: kind, Language: "fixture",
		Visibility: sema.Exported,
		Signature:  "signature of " + name,
		Doc:        "what " + name + " is for.",
		Snippet:    "the whole of " + name,
		Span: source.Span{
			Path:  "pkg/a.fx",
			Start: source.Position{Offset: start, Line: start},
			End:   source.Position{Offset: end, Line: end},
		},
	}
}

// held finds one declaration by name at the top of an answer.
func held(t *testing.T, in []tool.Declaration, name string) tool.Declaration {
	t.Helper()
	for _, d := range in {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("no declaration named %q", name)
	return tool.Declaration{}
}

func TestDeclared(t *testing.T) {
	t.Parallel()

	t.Run("nesting", func(t *testing.T) {
		t.Parallel()

		t.Run("puts a declaration inside the one whose bytes cover it", func(t *testing.T) {
			t.Parallel()
			// Containment is read off the spans, so it holds for a
			// grammar nobody has written a pattern for.
			got := tool.Declared([]sema.Symbol{
				covering("Store", sema.KindStruct, 0, 100),
				covering("Name", sema.KindField, 10, 20),
			}, tool.Names, nil)

			assert.Length(t, got, 1, "a declaration inside another is not also beside it")
			assert.Length(t, got[0].Members, 1, "what a declaration covers, it holds")
			assert.Equal(t, got[0].Members[0].Name, "Name",
				"the field is reached through the struct rather than pointed at from it")
		})

		t.Run("nests to any depth", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared([]sema.Symbol{
				covering("Outer", sema.KindStruct, 0, 100),
				covering("Inner", sema.KindStruct, 10, 60),
				covering("Leaf", sema.KindField, 20, 30),
			}, tool.Names, nil)

			assert.Length(t, got, 1, "only the outermost declaration is at the top")
			assert.Equal(t, got[0].Members[0].Name, "Inner", "each sits in the nearest that covers it")
			assert.Equal(t, got[0].Members[0].Members[0].Name, "Leaf",
				"each sits in the nearest that covers it")
		})

		t.Run("leaves two declarations that only overlap side by side", func(t *testing.T) {
			t.Parallel()
			// Overlap is not nesting, and a tree built from it would not
			// be one the source has.
			got := tool.Declared([]sema.Symbol{
				covering("First", sema.KindFunction, 0, 50),
				covering("Second", sema.KindFunction, 40, 90),
			}, tool.Names, nil)
			assert.Length(t, got, 2, "containment is nesting; overlap is not")
		})

		t.Run("does not nest across files", func(t *testing.T) {
			t.Parallel()
			wide := covering("Wide", sema.KindStruct, 0, 100)
			narrow := covering("Narrow", sema.KindStruct, 10, 20)
			narrow.Span.Path = "pkg/b.fx"

			got := tool.Declared([]sema.Symbol{wide, narrow}, tool.Names, nil)
			assert.Length(t, got, 2, "an offset means nothing across two files")
		})
	})

	t.Run("include", func(t *testing.T) {
		t.Parallel()

		binding := []sema.Symbol{
			covering("Get", sema.KindFunction, 0, 100),
			covering("id", sema.KindParameter, 5, 10),
			covering("local", sema.KindVariable, 20, 30),
			covering("fmt", sema.KindImport, 200, 210),
			covering("Store", sema.KindStruct, 300, 400),
		}

		t.Run("holds what a file offers and nothing scoped inside a body", func(t *testing.T) {
			t.Parallel()
			// An outline answers what a file offers. A parameter belongs
			// to a signature and a local binding leaves no scope, so
			// reporting either as a peer of the function costs a caller
			// context for a question it did not ask.
			got := tool.Declared(binding, tool.Names, nil)
			assert.Length(t, got, 2, "a function and a struct are what this file offers")
			assert.Empty(t, held(t, got, "Get").Members,
				"a parameter and a local are not members a caller can name")
		})

		t.Run("adds the imports when a caller asks", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(binding, tool.Names, []string{string(tool.IncludeImport)})
			assert.Length(t, got, 3, "what a file brings into scope is a question of its own")
			assert.Equal(t, held(t, got, "fmt").Kind, sema.KindImport,
				"an import is asked for by name and comes back as one")
		})

		t.Run("adds the bindings in a signature when a caller asks", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(binding, tool.Names, []string{string(tool.IncludeParameter)})
			assert.Length(t, held(t, got, "Get").Members, 1, "a parameter sits inside its callable")
		})

		t.Run("adds what is written inside a body when a caller asks", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(binding, tool.Names, []string{string(tool.IncludeLocal)})
			assert.Length(t, held(t, got, "Get").Members, 1, "a local sits inside its callable")
		})

		t.Run("adds every binding at once", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(binding, tool.Names, []string{string(tool.IncludeAll)})
			assert.Length(t, got, 3, "all is every name the file binds")
			assert.Length(t, held(t, got, "Get").Members, 2, "all is every name the file binds")
		})
	})

	t.Run("levels", func(t *testing.T) {
		t.Parallel()
		one := []sema.Symbol{covering("Store", sema.KindStruct, 0, 100)}

		t.Run("names carry where a declaration is and no more", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Names, nil)[0]
			assert.NotEmpty(t, got.Name, "a level a caller navigates by carries the line")
			assert.True(t, got.Line > 0, "a line is counted from one, as an editor counts")
			assert.Empty(t, got.Signature, "names is the level below signatures")
			assert.Empty(t, got.Doc, "names is the level below docs")
			assert.Nil(t, got.Span, "a span is what the write path slices, not what a reader navigates by")
		})

		t.Run("signatures carry how to call it and nothing of how it works", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Signatures, nil)[0]
			assert.NotEmpty(t, got.Signature, "this is the level an outline replaces reading the file at")
			assert.Empty(t, got.Doc, "signatures is the level below docs")
			assert.Empty(t, got.Snippet, "signatures holds nothing of how a declaration works")
		})

		t.Run("docs adds the documentation", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Docs, nil)[0]
			assert.NotEmpty(t, got.Doc, "docs is the level that answers what a declaration is for")
			assert.Empty(t, got.Snippet, "docs is the level below source")
		})

		t.Run("source adds the text and the bytes it covers", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Source, nil)[0]
			assert.NotEmpty(t, got.Snippet, "source is the level that answers what a declaration does")
			assert.NotNil(t, got.Span, "a caller slicing bytes needs the offsets, and only there")
		})

		t.Run("state a visibility only where it is not the common answer", func(t *testing.T) {
			t.Parallel()
			// A request returns exported declarations unless it asked
			// otherwise, so saying so on each is a word carrying nothing.
			exported := tool.Declared(one, tool.Signatures, nil)[0]
			assert.Equal(t, exported.Visibility, sema.VisibilityUnknown,
				"the common answer is left to the scope rather than repeated per item")

			hidden := covering("store", sema.KindStruct, 0, 100)
			hidden.Visibility = sema.Unexported
			got := tool.Declared([]sema.Symbol{hidden}, tool.Signatures, nil)[0]
			assert.Equal(t, got.Visibility, sema.Unexported,
				"a declaration that does not leave its unit says so")
		})
	})

	t.Run("an answer that found nothing", func(t *testing.T) {
		t.Parallel()

		t.Run("is an empty list rather than nothing at all", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(nil, tool.Names, nil)
			assert.NotNil(t, got, "a caller reads an absent list and an empty one the same way only by accident")
			assert.Length(t, got, 0, "nothing was found")
		})
	})
}
