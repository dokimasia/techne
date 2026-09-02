// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what the language's own formatter would change", func(t *testing.T) {
			t.Parallel()
			// gofmt behind gopls, the TypeScript formatter behind its
			// server. A caller gets what the language's tooling would
			// produce rather than what techne thinks it should look
			// like.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Format(t.Context(), []source.Path{"a.fake"})

			assert.NoError(t, err, "formatting a file the server formats succeeds")
			assert.Length(t, got.Items, 1, "one file to change")
			assert.Equal(t, got.Items[0].Kind, edit.ChangeEdit, "by rewriting ranges in it")
			assert.Length(t, got.Items[0].Edits, 1, "with the edits the formatter named")
		})

		t.Run("returns nothing for a file already written that way", func(t *testing.T) {
			t.Parallel()
			// A change with no edits would have the write path rewrite a
			// file to itself, and report that it had done something.
			got, err := serving(t, modeEmpty, map[string]string{"a.fake": content}).
				Format(t.Context(), []source.Path{"a.fake"})

			assert.NoError(t, err, "a file that needs nothing is not a fault")
			assert.Empty(t, got.Items, "and nothing is reported as changed")
		})

		t.Run("leaves a file of another language alone", func(t *testing.T) {
			t.Parallel()
			// A caller naming a mixed set gets each file from whoever
			// claims it, rather than a refusal for the whole set.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Format(t.Context(), []source.Path{"notes.md"})

			assert.NoError(t, err, "a path this language does not claim is not a fault")
			assert.Empty(t, got.Items, "and nothing of it is touched")
		})

		t.Run("declines where the server does not format", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, modeThin, map[string]string{"a.fake": content}).
				Format(t.Context(), []source.Path{"a.fake"})

			assert.ErrorIs(t, err, engine.ErrDecline,
				"another engine gets a turn, rather than the call breaking")
		})
	})
}
