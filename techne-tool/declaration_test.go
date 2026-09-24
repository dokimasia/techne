// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/tool"
)

// covering returns an exported declaration of pkg/a.fx named name, of kind, whose span covers
// the bytes from start to end, with a signature, documentation and source text.
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

// hidden returns the declaration of [covering] as unexported.
func hidden(name string, kind sema.Kind, start, end int) sema.Symbol {
	out := covering(name, kind, start, end)
	out.Visibility = sema.Unexported
	return out
}

// getOf returns the declaration of in named Get, and fails the test when none is.
func getOf(t *testing.T, in []tool.Declaration) tool.Declaration {
	t.Helper()
	for _, d := range in {
		if d.Name == "Get" {
			return d
		}
	}
	t.Fatal("no declaration named Get")
	return tool.Declaration{}
}

// names returns the name of each declaration of in, in order.
func names(in []tool.Declaration) []string {
	out := make([]string, 0, len(in))
	for _, one := range in {
		out = append(out, one.Name)
	}
	return out
}

func TestDeclaration(t *testing.T) {
	t.Parallel()

	bindings := []sema.Symbol{
		covering("Get", sema.KindFunction, 0, 100),
		covering("id", sema.KindParameter, 5, 10),
		covering("local", sema.KindVariable, 20, 30),
		covering("fmt", sema.KindImport, 200, 210),
		covering("Store", sema.KindStruct, 300, 400),
	}
	one := []sema.Symbol{covering("Store", sema.KindStruct, 0, 100)}

	t.Run("Declared", func(t *testing.T) {
		t.Parallel()

		t.Run("nests a declaration under the declaration that covers it", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared([]sema.Symbol{
				covering("Store", sema.KindStruct, 0, 100),
				covering("Name", sema.KindField, 10, 20),
			}, tool.Names, 0)
			assert.Equal(t, names(got), []string{"Store"}, "the declarations at the top level")
			assert.Equal(t, names(got[0].Members), []string{"Name"}, "the members of Store")
		})

		t.Run("nests a declaration under the smallest declaration that covers it", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared([]sema.Symbol{
				covering("Outer", sema.KindStruct, 0, 100),
				covering("Inner", sema.KindStruct, 10, 60),
				covering("Leaf", sema.KindField, 20, 30),
			}, tool.Names, 0)
			assert.Equal(t, names(got[0].Members), []string{"Inner"}, "the members of Outer")
			assert.Equal(t, names(got[0].Members[0].Members), []string{"Leaf"}, "the members of Inner")
		})

		t.Run("leaves two overlapping declarations at one level", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared([]sema.Symbol{
				covering("First", sema.KindFunction, 0, 50),
				covering("Second", sema.KindFunction, 40, 90),
			}, tool.Names, 0)
			assert.Equal(t, names(got), []string{"First", "Second"}, "the declarations at the top level")
		})

		t.Run("nests no declaration under a declaration of another file", func(t *testing.T) {
			t.Parallel()
			narrow := covering("Narrow", sema.KindStruct, 10, 20)
			narrow.Span.Path = "pkg/b.fx"
			got := tool.Declared([]sema.Symbol{covering("Wide", sema.KindStruct, 0, 100), narrow}, tool.Names, 0)
			assert.Equal(t, names(got), []string{"Wide", "Narrow"}, "the declarations at the top level")
		})

		t.Run("leaves out the bindings without include", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(bindings, tool.Names, 0)
			assert.Equal(t, names(got), []string{"Get", "Store"}, "the declarations at the top level")
			assert.Empty(t, getOf(t, got).Members, "the members of Get")
		})

		t.Run("returns the imports with BindImports", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(bindings, tool.Names, engine.BindImports)
			assert.Equal(t, names(got), []string{"Get", "fmt", "Store"}, "the declarations at the top level")
		})

		t.Run("returns the parameters with BindParameters", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(bindings, tool.Names, engine.BindParameters)
			assert.Equal(t, names(getOf(t, got).Members), []string{"id"}, "the members of Get")
		})

		t.Run("returns the local declarations with BindLocals", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(bindings, tool.Names, engine.BindLocals)
			assert.Equal(t, names(getOf(t, got).Members), []string{"local"}, "the members of Get")
		})

		t.Run("returns every binding with BindAll", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(bindings, tool.Names, engine.BindAll)
			assert.Equal(t, names(got), []string{"Get", "fmt", "Store"}, "the declarations at the top level")
			assert.Equal(t, names(getOf(t, got).Members), []string{"id", "local"}, "the members of Get")
		})

		t.Run("returns the name and the line at Names", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Names, 0)[0]
			assert.Equal(t, got.Name, "Store", "the name")
			assert.Equal(t, got.Line, 1, "the line counted from one")
			assert.Empty(t, got.Signature, "the signature")
			assert.Nil(t, got.Span, "the span")
		})

		t.Run("adds the signature at Signatures", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Signatures, 0)[0]
			assert.Equal(t, got.Signature, "signature of Store", "the signature")
			assert.Empty(t, got.Doc, "the documentation")
		})

		t.Run("adds the documentation at Docs", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Docs, 0)[0]
			assert.Equal(t, got.Doc, "what Store is for.", "the documentation")
			assert.Empty(t, got.Snippet, "the source text")
		})

		t.Run("adds the source text and the span at Source", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(one, tool.Source, 0)[0]
			assert.Equal(t, got.Snippet, "the whole of Store", "the source text")
			assert.NotNil(t, got.Span, "the span")
		})

		t.Run("states the visibility of an unexported declaration alone", func(t *testing.T) {
			t.Parallel()
			exported := tool.Declared(one, tool.Signatures, 0)[0]
			assert.Equal(t, exported.Visibility, sema.VisibilityUnknown, "the visibility of an exported declaration")
			unexported := tool.Declared([]sema.Symbol{hidden("store", sema.KindStruct, 0, 100)}, tool.Signatures, 0)[0]
			assert.Equal(t, unexported.Visibility, sema.Unexported, "the visibility of an unexported declaration")
		})

		t.Run("returns an empty list for no declarations", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared(nil, tool.Names, 0)
			assert.NotNil(t, got, "the list of declarations")
			assert.Length(t, got, 0, "the declarations")
		})
	})

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		mixed := []sema.Symbol{
			covering("Store", sema.KindStruct, 0, 100),
			hidden("helper", sema.KindFunction, 200, 300),
		}

		t.Run("leaves out an unexported declaration at every level", func(t *testing.T) {
			t.Parallel()
			for _, level := range tool.Levels() {
				got := tool.Narrow{}.Apply(tool.Declared(mixed, level, 0))
				assert.Equal(t, names(got), []string{"Store"}, "the declarations at "+string(level))
			}
		})

		t.Run("keeps an unexported declaration with Private at every level", func(t *testing.T) {
			t.Parallel()
			for _, level := range tool.Levels() {
				got := tool.Narrow{Private: true}.Apply(tool.Declared(mixed, level, 0))
				assert.Equal(t, names(got), []string{"Store", "helper"}, "the declarations at "+string(level))
			}
		})

		t.Run("keeps a member of a declaration that it leaves out", func(t *testing.T) {
			t.Parallel()
			got := tool.Narrow{Kind: sema.KindField}.Apply(tool.Declared([]sema.Symbol{
				covering("Store", sema.KindStruct, 0, 100),
				covering("size", sema.KindField, 10, 20),
				covering("Get", sema.KindMethod, 30, 40),
			}, tool.Names, 0))
			assert.Equal(t, names(got), []string{"Store"}, "the declarations at the top level")
			assert.Equal(t, names(got[0].Members), []string{"size"}, "the members of Store")
		})

		t.Run("keeps a declaration by its name with all its members", func(t *testing.T) {
			t.Parallel()
			got := tool.Narrow{Names: []string{"Store"}}.Apply(tool.Declared([]sema.Symbol{
				covering("Store", sema.KindStruct, 0, 100),
				covering("size", sema.KindField, 10, 20),
				covering("Other", sema.KindStruct, 200, 300),
			}, tool.Names, 0))
			assert.Equal(t, names(got), []string{"Store"}, "the declarations at the top level")
			assert.Equal(t, names(got[0].Members), []string{"size"}, "the members of Store")
		})

		t.Run("keeps the declarations of a prefix", func(t *testing.T) {
			t.Parallel()
			got := tool.Narrow{Prefix: "Sto"}.Apply(tool.Declared([]sema.Symbol{
				covering("Store", sema.KindStruct, 0, 100),
				covering("Other", sema.KindStruct, 200, 300),
			}, tool.Names, 0))
			assert.Equal(t, names(got), []string{"Store"}, "the declarations that start with Sto")
		})
	})
}
