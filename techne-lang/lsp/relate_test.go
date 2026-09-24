// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestRelate(t *testing.T) {
	t.Parallel()

	request := engine.Request{Scope: "a.fake"}

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("relates each use to the declaration that contains it", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("returns the source line of each use", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, got.Items[0].Via, "func (s *Store) Get() int { return s.size }",
				"the line of the first use")
		})

		t.Run("cuts the source line of a use on a long line", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Minified, map[string]string{"a.fake": lsptest.Bundle(100)}).
				Relate(t.Context(), request, declared("F0", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0 in a bundle")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the declarations that use F0")
			via := got.Items[0].Via
			assert.True(t, strings.HasPrefix(via, "…") && strings.HasSuffix(via, "return F0() }"),
				"the line of the use of F0 ends at the use: "+via)
			assert.True(t, len(via) <= lang.LineLimit+len("…"),
				"the line of the use of F0 is at most lang.LineLimit bytes and an ellipsis: "+via)
		})

		t.Run("returns the relations up to the limit of the request", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake", Limit: 1}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store with a limit of 1")
			assert.Equal(t, edges(got.Items), []string{"Get"}, "the declarations that use Store")
		})

		t.Run("counts the relations past the limit in a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake", Limit: 1}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store with a limit of 1")
			var notes []string
			for _, one := range got.Caveats {
				if one.Code == trust.CaveatTruncated {
					notes = append(notes, one.Note)
				}
			}
			assert.Equal(t, notes, []string{"1 of 2 relations returned"}, "the notes of the truncation caveats")
		})

		t.Run("reads no file of a relation past the limit", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			e := serving(t, lsptest.Minified, map[string]string{
				"a.fake": lsptest.Bundle(3), "b.fake": "package a\nfunc Caller() int { return F0() }\n",
			}, lsptest.RecordRequests(log))
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake", Limit: 1},
				declared("F0", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0 with a limit of 1")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the declarations that use F0")
			requests, err := os.ReadFile(log)
			assert.NoError(t, err, "the log of the requests")
			assert.Equal(t, strings.Count(string(requests), "textDocument/documentSymbol\n"), 1,
				"the requests for symbols, of a.fake only")
		})

		t.Run("returns the callers from the call hierarchy", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store.Get", sema.KindMethod), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Get")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the callers of Get")
		})

		t.Run("returns the call site of an outgoing call", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("After", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Relate of the calls of After")
			assert.Equal(t, edges(got.Items), []string{"Get"}, "the declarations that After calls")
			assert.Equal(t, got.Items[0].At.Path, source.Path("a.fake"), "the file of the call site")
			assert.Equal(t, got.Items[0].At.Start.Line, 8, "the line of the call site")
			assert.Equal(t, got.Items[0].At.Start.Column, 37, "the column of the call site")
			assert.Equal(t, got.Items[0].Via, "func After() int { return (&Store{}).Get() }",
				"the line of the call site")
		})

		t.Run("returns the call site in the file of the caller", func(t *testing.T) {
			t.Parallel()
			far := filepath.Join(lsptest.Workspace(t, map[string]string{"far.fake": lsptest.Content}), "far.fake")
			got, err := serving(t, lsptest.Default, sample(), lsptest.Outside(far)).Relate(t.Context(), request,
				declared("After", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Relate of the calls of After into "+far)
			assert.Equal(t, got.Items[0].At.Path, source.Path("a.fake"), "the file of the call site")
			assert.Equal(t, got.Items[0].At.Start.Line, 8, "the line of the call site")
		})

		t.Run("returns the implementations of a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ImplementedBy)
			assert.NoError(t, err, "Relate of the implementations of Store")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the implementations of Store")
		})

		t.Run("returns the supertypes of a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.Embeds)
			assert.NoError(t, err, "Relate of the supertypes of Store")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the supertypes of Store")
		})

		t.Run("returns the subtypes of a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.EmbeddedBy)
			assert.NoError(t, err, "Relate of the subtypes of Store")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the subtypes of Store")
		})

		t.Run("relates a use outside every declaration to its file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Unenclosed, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of a use on line 0")
			assert.Equal(t, got.Items[0].To.Kind, sema.KindFile, "the kind of the far end")
			assert.True(t, slices.Contains(sema.Kinds(), got.Items[0].To.Kind), "sema.Kinds contains the kind")
		})

		t.Run("relates a use in a file outside the workspace", func(t *testing.T) {
			t.Parallel()
			far := filepath.Join(lsptest.Workspace(t, map[string]string{"far.fake": lsptest.Content}), "far.fake")
			got, err := serving(t, lsptest.Default, sample(), lsptest.Outside(far)).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of uses in "+far)
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("declines imports", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.Imports)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("declines a declaration the server cannot call", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Uncallable, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.CalledBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "not a function", "the error of Relate")
		})

		t.Run("declines a question that the server does not answer in time", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Hangs)
			server.Answering = 300 * time.Millisecond
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()), server)
			_, err := e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "did not answer within 300ms", "the error of Relate")
		})

		t.Run("declines a declaration that no file declares", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Missing", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("skips a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"}).
				Relate(t.Context(), engine.Request{Scope: "."}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate in a scope without a file of the language")
			assert.True(t, got.Skipped, "Skipped of the answer")
		})
	})
}
