// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestCapable(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		for _, asked := range []struct {
			kind    sema.RelationKind
			request string
		}{
			{sema.ReferencedBy, "textDocument/references"},
			{sema.CalledBy, "the call hierarchy"},
			{sema.Embeds, "the type hierarchy"},
		} {
			t.Run("declines "+asked.request+" on a server that does not offer it", func(t *testing.T) {
				t.Parallel()
				_, err := serving(t, lsptest.Thin, sample()).Relate(t.Context(), engine.Request{Scope: "a.fake"},
					declared("Store", sema.KindStruct), asked.kind)
				assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
				assert.Contains(t, err.Error(), asked.request, "the error of Relate")
			})
		}
	})

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("renames without prepareRename on a server that does not offer it", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Thin, sample()))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake"}, "the files the rename changes")
		})
	})
}
