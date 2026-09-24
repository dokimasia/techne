// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the edits of the formatter", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Format(t.Context(), []source.Path{"a.fake"})
			assert.NoError(t, err, "Format of a.fake")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit}, "the changes of Format")
			assert.Equal(t, got.Items[0].Edits[0].New, "TYPE", "the text of the edit")
		})

		t.Run("returns no change for a formatted file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Empty, sample()).Format(t.Context(), []source.Path{"a.fake"})
			assert.NoError(t, err, "Format of a formatted file")
			assert.Empty(t, got.Items, "the changes of Format")
		})

		t.Run("skips paths of another language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Format(t.Context(), []source.Path{"notes.md"})
			assert.NoError(t, err, "Format of notes.md")
			assert.True(t, got.Skipped, "Skipped of the answer")
		})

		t.Run("declines a server without formatting", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Thin, sample()).Format(t.Context(), []source.Path{"a.fake"})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Format")
		})

		t.Run("returns a partial answer for a file larger than lang.Largest", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{
				"a.fake":   lsptest.Content,
				"big.fake": strings.Repeat("x", lang.Largest+1),
			}).Format(t.Context(), []source.Path{"a.fake", "big.fake"})
			assert.NoError(t, err, "Format of a large file")
			assert.Length(t, got.Items, 1, "the changes of Format")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, unreadIn(got.Caveats, "big.fake"), "the unread caveat names big.fake")
		})
	})
}
