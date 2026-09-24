// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/mock"
)

// id returns the ID of the declaration of kind named qualified in the unit src.
func id(qualified string, kind sema.Kind) sema.ID {
	return sema.NewID(mock.Language, "src", qualified, kind)
}

// holders returns the name of the declaration at the far end of each relation, in order.
func holders(edges []sema.Relation) []string {
	out := make([]string, 0, len(edges))
	for _, one := range edges {
		out = append(out, one.To.Name)
	}
	return out
}

// twins returns a workspace with two types that each declare a method Get.
func twins() fstest.MapFS {
	return fstest.MapFS{"src/a.mock": {Data: []byte(
		"type Store\n  method Get\n    use Item\ntype Cache\n  method Get\n    use Entry\ntype Item\ntype Entry\n")}}
}

// doubles returns a workspace with two functions of one ID in two files of one unit.
func doubles() fstest.MapFS {
	return fstest.MapFS{
		"src/a.mock": {Data: []byte("func Twice\n")},
		"src/b.mock": {Data: []byte("func Twice\n")},
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration of a use in another file", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Resolve(t.Context(), engine.Request{Scope: "src/client.mock"},
				source.Position{Line: 2, Column: 6})
			assert.NoError(t, err, "Resolve of the use of Store")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations of the use")
			assert.Equal(t, got.Items[0].Span.Path, source.Path("src/store.mock"), "the file of Store")
		})

		t.Run("returns the declaration at its own name", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Resolve(t.Context(), engine.Request{Scope: "src/store.mock"},
				source.Position{Line: 1, Column: 5})
			assert.NoError(t, err, "Resolve of the name of Store")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations of the name")
		})

		t.Run("returns no declaration for a position on a word of the language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Resolve(t.Context(), engine.Request{Scope: "src/store.mock"},
				source.Position{Line: 1, Column: 0})
			assert.NoError(t, err, "Resolve of the word type")
			assert.Empty(t, got.Items, "the declarations of the word type")
			assert.False(t, got.Skipped, "the skip of src/store.mock")
		})

		t.Run("reads the line and the column of a position from its offset", func(t *testing.T) {
			t.Parallel()
			content := string(workspace()["src/client.mock"].Data)
			got, err := built(t).Resolve(t.Context(), engine.Request{Scope: "src/client.mock"},
				source.Position{Offset: strings.LastIndex(content, "Store")})
			assert.NoError(t, err, "Resolve at the offset of Store")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations at the offset")
		})

		t.Run("returns a skipped result for a file of another language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Resolve(t.Context(), engine.Request{Scope: "notes.md"}, source.Position{})
			assert.NoError(t, err, "Resolve in notes.md")
			assert.True(t, got.Skipped, "the skip of notes.md")
		})

		t.Run("declines a directory with files of the language", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Resolve(t.Context(), engine.Request{Scope: "src"}, source.Position{})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve in src")
		})

		t.Run("declines a file of the language that the workspace does not contain", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Resolve(t.Context(), engine.Request{Scope: "src/absent.mock"}, source.Position{})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve in src/absent.mock")
		})

		t.Run("declines a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			fsys := workspace()
			fsys["big.mock"] = &fstest.MapFile{Data: bytes.Repeat([]byte("x"), lang.Largest+1)}
			_, err := over(t, fsys).Resolve(t.Context(), engine.Request{Scope: "big.mock"}, source.Position{})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve in big.mock")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration that contains each use of a declaration", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Store", sema.KindType), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of Store")
			assert.Equal(t, holders(got.Items), []string{"Client", "Get", "New"}, "the declarations that use Store")
		})

		t.Run("returns the site of each use with its text", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Store", sema.KindType), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of Store")
			assert.Length(t, got.Items, 3, "the uses of Store")
			for _, one := range got.Items {
				content := workspace()[string(one.At.Path)].Data
				assert.Equal(t, covered(content, one.At), "Store", "the site in "+string(one.At.Path))
				assert.Equal(t, one.Via, "use Store", "the text of the site in "+string(one.At.Path))
			}
		})

		t.Run("returns the kind that the request names", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Store", sema.KindType), sema.CalledBy)
			assert.NoError(t, err, "Relate of Store")
			assert.Length(t, got.Items, 3, "the uses of Store")
			for _, one := range got.Items {
				assert.Equal(t, one.Kind, sema.CalledBy, "the kind of the relation from "+one.To.Name)
			}
		})

		t.Run("returns the declarations that the uses inside a declaration name", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Client", sema.KindFunction), sema.References)
			assert.NoError(t, err, "Relate of Client")
			assert.Equal(t, holders(got.Items), []string{"New", "Store"}, "the declarations that Client uses")
		})

		t.Run("returns the uses inside the declaration that the ID names", func(t *testing.T) {
			t.Parallel()
			got, err := over(t, twins()).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Store.Get", sema.KindMethod), sema.References)
			assert.NoError(t, err, "Relate of Store.Get")
			assert.Equal(t, holders(got.Items), []string{"Item"}, "the declarations that Store.Get uses")
		})

		t.Run("returns no relation for a kind without an edge", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Store", sema.KindType), sema.Embeds)
			assert.NoError(t, err, "Relate of Store")
			assert.Empty(t, got.Items, "the types that Store embeds")
		})

		t.Run("returns a skipped result for a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Relate(t.Context(), engine.Request{Scope: "notes.md"},
				id("Store", sema.KindType), sema.ReferencedBy)
			assert.NoError(t, err, "Relate in notes.md")
			assert.True(t, got.Skipped, "the skip of notes.md")
		})

		t.Run("declines an ID that no declaration has", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Absent", sema.KindType), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate of Absent")
		})

		t.Run("refuses an ID that two declarations have", func(t *testing.T) {
			t.Parallel()
			_, err := over(t, doubles()).Relate(t.Context(), engine.Request{Scope: "src"},
				id("Twice", sema.KindFunction), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Relate of Twice")
		})
	})
}
