// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"strings"
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

		t.Run("adds the first sentence of the documentation at Summaries", func(t *testing.T) {
			t.Parallel()
			store := covering("Store", sema.KindStruct, 0, 100)
			store.Doc = "Store keeps items\nby name. It is safe to share."
			got := tool.Declared([]sema.Symbol{store}, tool.Summaries, 0)[0]
			assert.Equal(t, got.Summary, "Store keeps items by name.", "the summary")
			assert.Empty(t, got.Signature, "the signature")
			assert.Empty(t, got.Doc, "the documentation")
		})

		t.Run("cuts a first sentence longer than 160 bytes at a space", func(t *testing.T) {
			t.Parallel()
			store := covering("Store", sema.KindStruct, 0, 100)
			store.Doc = strings.Repeat("word ", 40) + "end."
			got := tool.Declared([]sema.Symbol{store}, tool.Summaries, 0)[0].Summary
			assert.HasSuffix(t, got, "word…", "the end of the summary")
			assert.True(t, len(got) <= 160+len("…"), "the length of the summary: "+got)
		})

		t.Run("ends the summary before a list item", func(t *testing.T) {
			t.Parallel()
			pkg := covering("c", sema.KindPackage, 0, 100)
			pkg.Doc = "Package c declares the C language:\n  - its extensions\n  - its grammar"
			got := tool.Declared([]sema.Symbol{pkg}, tool.Summaries, 0)[0]
			assert.Equal(t, got.Summary, "Package c declares the C language:", "the summary")
		})

		t.Run("ends the summary at the end of the first paragraph", func(t *testing.T) {
			t.Parallel()
			pkg := covering("c", sema.KindPackage, 0, 100)
			pkg.Doc = "Package c reads C\n\nIt parses."
			got := tool.Declared([]sema.Symbol{pkg}, tool.Summaries, 0)[0]
			assert.Equal(t, got.Summary, "Package c reads C", "the summary")
		})

		t.Run("returns the type of a field at Names", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared([]sema.Symbol{
				covering("Store", sema.KindStruct, 0, 100), covering("size", sema.KindField, 10, 20),
			}, tool.Names, 0)
			assert.Equal(t, got[0].Members[0].Signature, "signature of size", "the signature of size")
			assert.Empty(t, got[0].Signature, "the signature of Store")
		})

		t.Run("leaves out a member without a summary or a type at Summaries", func(t *testing.T) {
			t.Parallel()
			method := covering("Get", sema.KindMethod, 10, 20)
			method.Doc = ""
			got := tool.Declared([]sema.Symbol{covering("Store", sema.KindInterface, 0, 100), method},
				tool.Summaries, 0)
			assert.Empty(t, got[0].Members, "the members of Store")
		})

		t.Run("keeps a member with a summary at Summaries", func(t *testing.T) {
			t.Parallel()
			got := tool.Declared([]sema.Symbol{
				covering("Store", sema.KindInterface, 0, 100), covering("Get", sema.KindMethod, 10, 20),
			}, tool.Summaries, 0)
			assert.Equal(t, names(got[0].Members), []string{"Get"}, "the members of Store")
		})

		t.Run("adds no summary at Signatures", func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, tool.Declared(one, tool.Signatures, 0)[0].Summary, "the summary")
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

		t.Run("counts the lines and the columns of the span from one", func(t *testing.T) {
			t.Parallel()
			store := covering("Store", sema.KindStruct, 0, 100)
			store.Span.Start.Column, store.Span.End.Column = 0, 1
			got := tool.Declared([]sema.Symbol{store}, tool.Source, 0)[0]
			assert.Equal(t, *got.Span, tool.Extent{
				Start: tool.Place{Line: 1, Column: 1, Offset: 0},
				End:   tool.Place{Line: 101, Column: 2, Offset: 100},
			}, "the span")
			assert.Equal(t, got.Span.Start.Line, got.Line, "the first line of the span and the line")
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

	t.Run("Resolved", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps a parameter at the top level without include", func(t *testing.T) {
			t.Parallel()
			got := tool.Resolved([]sema.Symbol{covering("ctx", sema.KindParameter, 0, 3)}, tool.Names, 0)
			assert.Equal(t, names(got), []string{"ctx"}, "the declarations at the top level")
		})

		t.Run("leaves out a member that include does not select", func(t *testing.T) {
			t.Parallel()
			got := tool.Resolved([]sema.Symbol{
				covering("Wait", sema.KindFunction, 0, 100),
				covering("ctx", sema.KindParameter, 10, 13),
			}, tool.Names, 0)
			assert.Equal(t, names(got), []string{"Wait"}, "the declarations at the top level")
			assert.Empty(t, got[0].Members, "the members of Wait")
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

		t.Run("keeps a declaration by its qualified name", func(t *testing.T) {
			t.Parallel()
			method := covering("Time", sema.KindMethod, 200, 300)
			method.ID = sema.NewID("fixture", "pkg", "Instant.Time", sema.KindMethod)
			other := covering("Time", sema.KindFunction, 400, 500)
			other.ID = sema.NewID("fixture", "pkg", "Time", sema.KindFunction)
			got := tool.Narrow{Names: []string{"Instant.Time"}}.Apply(tool.Declared([]sema.Symbol{
				covering("Instant", sema.KindStruct, 0, 100), method, other,
			}, tool.Names, 0))
			assert.Length(t, got, 1, "the declarations of Instant.Time")
			assert.Equal(t, got[0].Kind, sema.KindMethod, "the kind of Instant.Time")
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
