// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

func TestOutline(t *testing.T) {
	t.Parallel()

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the server said the file declares", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining a file the server answers about succeeds")
			assert.Equal(t, names(got.Items), []string{"Store", "size", "Get", "After"},
				"every declaration the server named, in the order it named them")
		})

		t.Run("keeps the containment the server reported", func(t *testing.T) {
			t.Parallel()
			// Working it out again from spans would disagree with the
			// server wherever a language nests differently from how it
			// is written, which is what the hierarchical shape is for.
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			size, found := named(got.Items, "size")
			assert.True(t, found, "the member is there")
			assert.NotEmpty(t, string(size.Parent), "and sits inside what holds it")

			method, found := named(got.Items, "Get")
			assert.True(t, found, "the method is there")
			assert.Empty(t, string(method.Parent),
				"and is where the server put it, which for this language is the top level")
		})

		t.Run("reads a name the server qualified", func(t *testing.T) {
			t.Parallel()
			// gopls writes a method as (*Store).Get, which is how it is
			// declared and not what it is called. A caller searches for
			// the name.
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			_, found := named(got.Items, "Get")
			assert.True(t, found, "the method is named Get rather than (*Store).Get")
		})

		t.Run("drops a kind that declares nothing", func(t *testing.T) {
			t.Parallel()
			// The same request outlines a JSON document, so the protocol
			// names strings and keys as well as declarations. A caller
			// filtering an outline would otherwise meet entries it
			// cannot act on.
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			for _, one := range got.Items {
				assert.NotEqual(t, one.Kind, sema.KindUnknown,
					"a kind this vocabulary does not carry is left out, not reported as unknown")
			}
			assert.Length(t, got.Items, 4, "the string the fake reported is not among them")
		})

		t.Run("cuts a snippet the span actually covers", func(t *testing.T) {
			t.Parallel()
			// The protocol carries a line and a character and no offset,
			// and the offset is the authoritative coordinate here.
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			store, _ := named(got.Items, "Store")
			assert.HasPrefix(t, store.Snippet, "type Store struct {",
				"the snippet is the bytes the span names")
			assert.Equal(t, store.Signature, "type Store struct {",
				"and the signature is the line the name is on")
		})

		t.Run("counts characters the way the protocol does", func(t *testing.T) {
			t.Parallel()
			// The protocol measures in UTF-16 code units and this
			// vocabulary measures bytes. The server names a span from
			// unit 17 to 22, which is bytes 19 to 25 of the line; read
			// as bytes it is "e Stör", two to the left and cutting a
			// character in half. An edit computed from that writes over
			// half a rune.
			got, err := serving(t, "unicode", map[string]string{"a.fake": unicode}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			assert.Length(t, got.Items, 1, "the file declares one thing")
			assert.Equal(t, got.Items[0].Snippet, "Störe",
				"the span covers the name, not the two bytes before it")
			assert.Equal(t, got.Items[0].Span.Start.Offset, 30,
				"counted in bytes from the start of the file")
		})

		t.Run("says it read nothing where the scope holds none of its files", func(t *testing.T) {
			t.Parallel()
			// A directory with none of this language in it says nothing
			// about the language, and must not lower what an engine
			// beside it is worth.
			got, err := serving(t, "", map[string]string{"notes.md": "# notes\n"}).
				Outline(t.Context(), engine.Request{Scope: "."})

			assert.NoError(t, err, "a scope with nothing to read is not a fault")
			assert.True(t, got.Skipped, "and the engine says it read nothing")
			assert.Empty(t, got.Items, "having found nothing to find")
		})

		t.Run("carries the caveat every resolved answer carries", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			var carried bool
			for _, c := range got.Caveats {
				carried = carried || c.Code == trust.CaveatDynamic
			}
			assert.True(t, carried,
				"a type checker still sees nothing of what is assembled at run time")
		})

		t.Run("answers an empty file with nothing rather than a fault", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "empty", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "a file declaring nothing is an answer")
			assert.Empty(t, got.Items, "and the answer is nothing")
			assert.False(t, got.Skipped, "the file was read, which is different from not reading one")
		})
	})
}
