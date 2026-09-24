// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("declines an operation without a request", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.InlineVariable,
				edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
				edit.Args{})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Plan for inline.variable")
		})

		t.Run("refuses a plan that changes a file outside the workspace", func(t *testing.T) {
			t.Parallel()
			far := filepath.Join(lsptest.Workspace(t, map[string]string{"far.fake": lsptest.Content}), "far.fake")
			_, err := renameWith(t, serving(t, lsptest.Default, sample(), lsptest.Outside(far)))
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "outside the workspace", "the error of Plan")
		})

		t.Run("skips a span in a file of another language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Plan(t.Context(),
				engine.Request{Scope: "notes.md"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "notes.md"}},
				edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of a span in notes.md")
			assert.True(t, got.Skipped, "Skipped of the plan")
		})
	})
}
