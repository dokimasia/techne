// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/mock"
)

// names returns the name of each declaration, in order.
func names(items []sema.Symbol) []string {
	out := make([]string, 0, len(items))
	for _, one := range items {
		out = append(out, one.Name)
	}
	return out
}

// declared returns the first declaration of items named name, and fails the test when no
// declaration is.
func declared(t *testing.T, items []sema.Symbol, name string) sema.Symbol {
	t.Helper()
	i := slices.IndexFunc(items, func(one sema.Symbol) bool { return one.Name == name })
	assert.True(t, i >= 0, "a declaration named "+name)
	return items[i]
}

// codes returns the code of each caveat, in order.
func codes(caveats []trust.Caveat) []trust.CaveatCode {
	var out []trust.CaveatCode
	for _, one := range caveats {
		out = append(out, one.Code)
	}
	return out
}

func TestOutline(t *testing.T) {
	t.Parallel()

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of a scope in path order", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "Outline of src")
			assert.Equal(t, names(got.Items), []string{"Client", "Store", "size", "Get", "New"},
				"the declarations of src")
			assert.False(t, got.Skipped, "the skip of src")
		})

		t.Run("spans a declaration over the lines nested under it", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src/store.mock"})
			assert.NoError(t, err, "Outline of src/store.mock")
			assert.Equal(t, declared(t, got.Items, "Store").Snippet,
				"type Store\n  field size\n  method Get\n    use Store", "the source text of Store")
		})

		t.Run("starts the span of a nested declaration after its indentation", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src/store.mock"})
			assert.NoError(t, err, "Outline of src/store.mock")
			content := string(workspace()["src/store.mock"].Data)
			assert.Equal(t, declared(t, got.Items, "size").Span.Start,
				source.Position{Offset: strings.Index(content, "field size"), Line: 2, Column: 2},
				"the start of size")
		})

		t.Run("qualifies a member by the declaration that contains it", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src/store.mock"})
			assert.NoError(t, err, "Outline of src/store.mock")
			size := declared(t, got.Items, "size")
			assert.Equal(t, size.ID, sema.NewID(mock.Language, "src", "Store.size", sema.KindField), "the ID of size")
			assert.Equal(t, size.Parent, sema.NewID(mock.Language, "src", "Store", sema.KindType),
				"the parent of size")
		})

		t.Run("qualifies a declaration by the nearest declaration above it at a shallower depth", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"a.mock": {Data: []byte("type A\n  method M\ntype B\n    field x\n")}}
			got, err := over(t, fsys).Outline(t.Context(), engine.Request{Scope: "a.mock"})
			assert.NoError(t, err, "Outline of a.mock")
			x := declared(t, got.Items, "x")
			assert.Equal(t, x.ID, sema.NewID(mock.Language, "", "B.x", sema.KindField), "the ID of x")
			assert.Equal(t, x.Parent, sema.NewID(mock.Language, "", "B", sema.KindType), "the parent of x")
		})

		t.Run("returns the documentation above a declaration", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src/store.mock"})
			assert.NoError(t, err, "Outline of src/store.mock")
			assert.Equal(t, declared(t, got.Items, "Store").Doc, "Store maps a name to an item.",
				"the documentation of Store")
		})

		t.Run("leaves out a test file without Tests", func(t *testing.T) {
			t.Parallel()
			fsys := workspace()
			fsys["src/store_test.mock"] = &fstest.MapFile{Data: []byte("func TestStore\n  use Store\n")}
			got, err := over(t, fsys).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "Outline of src")
			assert.Equal(t, names(got.Items), []string{"Client", "Store", "size", "Get", "New"},
				"the declarations of src without tests")
		})

		t.Run("returns a test file with Tests", func(t *testing.T) {
			t.Parallel()
			fsys := workspace()
			fsys["src/store_test.mock"] = &fstest.MapFile{Data: []byte("func TestStore\n  use Store\n")}
			got, err := over(t, fsys).Outline(t.Context(), engine.Request{Scope: "src", Tests: true})
			assert.NoError(t, err, "Outline of src")
			assert.Equal(t, names(got.Items), []string{"Client", "Store", "size", "Get", "New", "TestStore"},
				"the declarations of src with tests")
		})

		t.Run("returns a skipped result for a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "notes.md"})
			assert.NoError(t, err, "Outline of notes.md")
			assert.True(t, got.Skipped, "the skip of notes.md")
			assert.Empty(t, got.Items, "the declarations of notes.md")
		})

		t.Run("returns a partial answer for a scope with a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			fsys := workspace()
			fsys["big.mock"] = &fstest.MapFile{Data: bytes.Repeat([]byte("x"), lang.Largest+1)}
			got, err := over(t, fsys, mock.At(trust.Syntactic)).Outline(t.Context(), engine.Request{Scope: "."})
			assert.NoError(t, err, "Outline of the root")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the outline")
			assert.Equal(t, got.Caveats, []trust.Caveat{{
				Code:  trust.CaveatUnread,
				Note:  fmt.Sprintf("larger than %d bytes, so not read", lang.Largest),
				Paths: []source.Path{"big.mock"},
			}}, "the caveats of the outline")
		})

		t.Run("returns the dynamic caveat at the resolved tier", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "Outline of src")
			assert.Equal(t, codes(got.Caveats), []trust.CaveatCode{trust.CaveatDynamic}, "the caveats of the outline")
		})

		t.Run("returns no caveat below the resolved tier", func(t *testing.T) {
			t.Parallel()
			got, err := built(t, mock.At(trust.Syntactic)).Outline(t.Context(), engine.Request{Scope: "src"})
			assert.NoError(t, err, "Outline of src")
			assert.Empty(t, got.Caveats, "the caveats of the outline")
		})
	})

	t.Run("Search", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an exact match before a prefix match before a loose match", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"a.mock": {Data: []byte("func Gets\nfunc Forget\nfunc Get\n")}}
			got, err := over(t, fsys).Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Text: "get"})
			assert.NoError(t, err, "Search of get")
			assert.Equal(t, names(got.Items), []string{"Get", "Gets", "Forget"}, "the matches of get")
		})

		t.Run("returns a declaration whose documentation contains the text", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"}, engine.Query{Text: "item"})
			assert.NoError(t, err, "Search of item")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the matches of item")
		})

		t.Run("leaves out an unexported declaration without Private", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"}, engine.Query{Text: "size"})
			assert.NoError(t, err, "Search of size")
			assert.Empty(t, got.Items, "the matches of size without Private")
		})

		t.Run("returns an unexported declaration with Private", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "size", Private: true})
			assert.NoError(t, err, "Search of size")
			assert.Equal(t, names(got.Items), []string{"size"}, "the matches of size with Private")
		})

		t.Run("returns the declarations of the kind of the query", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "e", Kind: sema.KindFunction, Private: true})
			assert.NoError(t, err, "Search of functions")
			assert.Equal(t, names(got.Items), []string{"Client", "New"}, "the functions that contain e")
		})

		t.Run("returns a truncation caveat for more matches than the limit", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "e", Private: true, Limit: 1})
			assert.NoError(t, err, "Search with a limit")
			assert.Equal(t, names(got.Items), []string{"Client"}, "the matches within the limit")
			assert.Equal(t, got.Caveats[len(got.Caveats)-1], trust.Caveat{
				Code: trust.CaveatTruncated,
				Note: "1 of 5 matches returned",
			}, "the truncation caveat")
		})

		t.Run("returns no match for a name that nothing declares", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Search(t.Context(), engine.Request{Scope: "src"},
				engine.Query{Text: "Absent", Private: true})
			assert.NoError(t, err, "Search of Absent")
			assert.Empty(t, got.Items, "the matches of Absent")
			assert.False(t, got.Skipped, "the skip of src")
		})

		t.Run("leaves out a local declaration without BindLocals", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"a.mock": {Data: []byte("func Fetch\n  var Found\n")}}
			got, err := over(t, fsys).Search(t.Context(), engine.Request{Scope: "."},
				engine.Query{Text: "f", Private: true})
			assert.NoError(t, err, "Search of f")
			assert.Equal(t, names(got.Items), []string{"Fetch"}, "the matches of f without locals")
		})

		t.Run("returns a local declaration with BindLocals", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"a.mock": {Data: []byte("func Fetch\n  var Found\n")}}
			got, err := over(t, fsys).Search(t.Context(), engine.Request{Scope: "."},
				engine.Query{Text: "f", Private: true, Include: engine.BindLocals})
			assert.NoError(t, err, "Search of f")
			assert.Equal(t, names(got.Items), []string{"Fetch", "Found"}, "the matches of f with locals")
		})

		t.Run("applies Include before the limit", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"a.mock": {Data: []byte("func Getter\n  var get\n")}}
			got, err := over(t, fsys).Search(t.Context(), engine.Request{Scope: "."},
				engine.Query{Text: "get", Private: true, Limit: 1})
			assert.NoError(t, err, "Search of get")
			assert.Equal(t, names(got.Items), []string{"Getter"}, "the match within the limit")
			assert.NotContains(t, codes(got.Caveats), trust.CaveatTruncated, "the caveats of one match")
		})
	})
}
