// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// pointed returns the declaration that Resolve returns for a position in a.fake, from the
// scripted server in the Pointed mode.
func pointed(t *testing.T, line, column int) sema.Symbol {
	t.Helper()
	got, err := serving(t, lsptest.Pointed, sample()).Resolve(t.Context(),
		engine.Request{Scope: "a.fake"}, source.Position{Line: line, Column: column})
	assert.NoError(t, err, "Resolve in the Pointed mode")
	assert.Length(t, got.Items, 1, "the declarations at the position")
	return got.Items[0]
}

// implementing returns the declaration that Resolve returns for a position in a.fake, from the
// scripted server in the Impls mode over [lsptest.Twins].
func implementing(t *testing.T, line, column int) sema.Symbol {
	t.Helper()
	got, err := serving(t, lsptest.Impls, map[string]string{"a.fake": lsptest.Twins}).Resolve(t.Context(),
		engine.Request{Scope: "a.fake"}, source.Position{Line: line, Column: column})
	assert.NoError(t, err, "Resolve in the Impls mode")
	assert.Length(t, got.Items, 1, "the declarations at the position")
	return got.Items[0]
}

// wrapping returns the declaration that Resolve returns for a position in a.fake, from the
// scripted server in the Wrapped mode.
func wrapping(t *testing.T, line, column int) sema.Symbol {
	t.Helper()
	got, err := serving(t, lsptest.Wrapped, sample()).Resolve(t.Context(),
		engine.Request{Scope: "a.fake"}, source.Position{Line: line, Column: column})
	assert.NoError(t, err, "Resolve in the Wrapped mode")
	assert.Length(t, got.Items, 1, "the declarations at the position")
	return got.Items[0]
}

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a method name without its receiver", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 6, 16).Name, "Get", "the name of (*Store).Get")
		})

		t.Run("returns a name without its parameters", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 8, 5).Name, "After", "the name of After(java.lang.String) : int")
		})

		t.Run("qualifies a member by the symbol that contains it", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 3, 1).ID.Name(), "Store.size", "the qualified name of size")
		})

		t.Run("qualifies a method by the receiver in its reported name", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 6, 16).ID.Name(), "Store.Get", "the qualified name of (*Store).Get")
		})

		t.Run("returns two identities for two methods of one name", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Receivers, map[string]string{"a.fake": lsptest.Twins})
			var ids []sema.ID
			for _, line := range []int{4, 8} {
				at := source.Position{Line: line, Column: 16}
				got, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, at)
				assert.NoError(t, err, "Resolve of the Get on line "+strconv.Itoa(line))
				assert.Length(t, got.Items, 1, "the declarations of the Get on line "+strconv.Itoa(line))
				ids = append(ids, got.Items[0].ID)
			}
			assert.Equal(t, []string{ids[0].Name(), ids[1].Name()}, []string{"Store.Get", "Cache.Get"},
				"the qualified names of the two methods")
		})

		t.Run("returns an impl block as an implementation of its type", func(t *testing.T) {
			t.Parallel()
			got := implementing(t, 4, 9)
			assert.Equal(t, got.Kind, sema.KindImplementation, "the kind of impl Getter for Store<T>")
			assert.Equal(t, got.Name, "Store", "the name of impl Getter for Store<T>")
		})

		t.Run("qualifies a method of an impl block by the type it implements", func(t *testing.T) {
			t.Parallel()
			got := implementing(t, 4, 16)
			assert.Equal(t, got.ID.Name(), "Store.Get", "the qualified name of the Get in impl Getter for Store<T>")
			assert.Equal(t, got.Parent, declared("Store", sema.KindImplementation), "the parent of that Get")
		})

		t.Run("returns a method inside a symbol that declares nothing", func(t *testing.T) {
			t.Parallel()
			got := implementing(t, 8, 16)
			assert.Equal(t, got.ID.Name(), "Get", "the qualified name of the Get in impl Getter for &Cache")
			assert.Equal(t, got.Parent, sema.ID(""), "the parent of that Get")
		})

		t.Run("qualifies a declaration inside a File symbol without the file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, wrapping(t, 6, 16).ID.Name(), "Shop.Get", "the qualified name of Get")
		})

		t.Run("reads the children of an anonymous function in its place", func(t *testing.T) {
			t.Parallel()
			got := wrapping(t, 8, 5)
			assert.Equal(t, got.ID.Name(), "After", "the qualified name of After")
			assert.Equal(t, got.Parent, sema.ID(""), "the parent of After")
		})

		t.Run("returns the innermost declaration at a position", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 3, 1).Name, "size", "the declaration at line 3, column 1")
		})

		t.Run("returns a member with its type as parent", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 3, 1).Parent, declared("Store", sema.KindStruct), "the parent of size")
		})

		t.Run("returns the line of the name as the signature", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pointed(t, 2, 5).Signature, "type Store struct {", "the signature of Store")
		})

		t.Run("drops a kind that declares nothing", func(t *testing.T) {
			t.Parallel()
			got := pointed(t, 0, 0)
			assert.Equal(t, got.Name, "a", "the declaration at line 0, column 0")
			assert.Equal(t, got.Kind, sema.KindPackage, "the kind of the package clause")
		})

		t.Run("reads a flat list of declarations", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Flat, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve with a flat list of declarations")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
			assert.Equal(t, got.Items[0].Signature, "type Store struct {", "the signature of Store")
		})

		t.Run("cuts the signature of a declaration on a long line", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Minified, map[string]string{"a.fake": lsptest.Bundle(100)}).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Line: 1, Column: 5})
			assert.NoError(t, err, "Resolve of F0 in a bundle")
			assert.Equal(t, names(got.Items), []string{"F0"}, "the declarations that F0 denotes")
			signature := got.Items[0].Signature
			assert.True(t, strings.HasPrefix(signature, "func F0() int { return 0 }; func F1()"),
				"the signature of F0 starts at its line: "+signature)
			assert.True(t, len(signature) <= lang.LineLimit+len("…"),
				"the signature of F0 is at most lang.LineLimit bytes and an ellipsis: "+signature)
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("finds a declaration by name when the kind differs", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of Store under another kind")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("finds a member by its qualified name when the kind differs", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Receivers, map[string]string{"a.fake": lsptest.Twins}).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Cache.Get", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Cache.Get under another kind")
		})

		t.Run("declines a base name that two members share", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Receivers, map[string]string{"a.fake": lsptest.Twins}).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Queue.Get", sema.KindMethod), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate of the uses of Queue.Get")
		})

		t.Run("finds a declaration whose ID leaves out the namespace of the server", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Wrapped, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store, which the server reports as Shop.Store")
		})

		t.Run("finds a declaration whose ID adds a namespace to the server's", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Wrapped, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Root.Shop.Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Root.Shop.Store, which the server reports as Shop.Store")
		})

		t.Run("finds a member by its base name when the containers differ", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Cache.Get", sema.KindMethod), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Cache.Get")
		})

		t.Run("qualifies a flat symbol by its container name", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Flat, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store in the Flat mode")
			assert.NotEmpty(t, got.Items, "the uses of Store")
			assert.Equal(t, got.Items[0].To.ID.Name(), "Store.Get",
				"the qualified name of the far end of the first use")
		})

		t.Run("relates a use in a file larger than lang.Largest to the file", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{
				"a.fake": lsptest.Content, "big.fake": strings.Repeat("x", lang.Largest+1),
			})
			e := lsptest.Engine(t, root,
				lsptest.Server(lsptest.Default, lsptest.Outside(filepath.Join(root, "big.fake"))))

			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with uses in big.fake")
			assert.Equal(t, edges(got.Items), []string{"big.fake", "big.fake"}, "the far ends of the uses")
		})

		t.Run("reads the symbols of a file once per call", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			got, err := serving(t, lsptest.Default, sample(), lsptest.RecordRequests(log)).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
			requests, err := os.ReadFile(log)
			assert.NoError(t, err, "the log of the requests")
			assert.Equal(t, strings.Count(string(requests), "textDocument/documentSymbol\n"), 1,
				"the requests for the symbols of a.fake")
		})

		t.Run("keeps the line of a use in a file larger than lang.Largest", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{
				"a.fake": lsptest.Content, "big.fake": strings.Repeat("x", lang.Largest+1),
			})
			e := lsptest.Engine(t, root,
				lsptest.Server(lsptest.Default, lsptest.Outside(filepath.Join(root, "big.fake"))))

			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with uses in big.fake")
			assert.Equal(t, got.Items[0].At, source.Span{
				Path: "big.fake", Start: source.Position{Line: 6}, End: source.Position{Line: 6},
			}, "the site of the first use, on the line that the server reported")
		})
	})
}
