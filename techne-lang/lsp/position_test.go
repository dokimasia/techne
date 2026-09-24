// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// renamedAt plans a rename of Store in a.fake over a workspace of files, and returns the
// plan. The scripted server responds to the rename with the workspace edit answer.
func renamedAt(t *testing.T, files map[string]string, answer string) (engine.Result[edit.Change], error) {
	t.Helper()
	return renameWith(t, serving(t, lsptest.Default, files, lsptest.Renames(answer)))
}

// renameWith plans a rename of Store in a.fake to Vault with e.
func renameWith(t *testing.T, e *lsp.Engine) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
		edit.Target{Kind: edit.TargetSymbol, Symbol: declared("Store", sema.KindStruct)},
		edit.Args{edit.ArgNewName: "Vault"})
}

// spanned plans a rename to Vault of the declaration at start in a.fake with e.
func spanned(t *testing.T, e *lsp.Engine, start source.Position) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
		edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "a.fake", Start: start}},
		edit.Args{edit.ArgNewName: "Vault"})
}

// applied returns content with the edits of the one change of changes applied.
func applied(t *testing.T, content string, changes []edit.Change) string {
	t.Helper()
	assert.Length(t, changes, 1, "the changes of the plan")
	out, err := edit.Apply([]byte(content), changes[0].Edits)
	assert.NoError(t, err, "edit.Apply of the change")
	return string(out)
}

func TestPosition(t *testing.T) {
	t.Parallel()

	emoji := map[string]string{"a.fake": lsptest.Emoji}

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("converts an offset to UTF-16 code units", func(t *testing.T) {
			t.Parallel()
			got, err := spanned(t, serving(t, lsptest.Strict, emoji), source.Position{Offset: 30})
			assert.NoError(t, err, "Plan at byte 30")
			assert.Length(t, got.Items, 1, "the changes of the plan")
		})

		t.Run("converts a line and a byte column to UTF-16 code units", func(t *testing.T) {
			t.Parallel()
			got, err := spanned(t, serving(t, lsptest.Strict, emoji), source.Position{Line: 2, Column: 19})
			assert.NoError(t, err, "Plan at line 2, column 19")
			assert.Length(t, got.Items, 1, "the changes of the plan")
		})

		t.Run("refuses a position the server does not name", func(t *testing.T) {
			t.Parallel()
			_, err := spanned(t, serving(t, lsptest.Strict, emoji), source.Position{Line: 2, Column: 0})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan at line 2, column 0")
		})

		t.Run("applies an edit at character 0 of the line after the last line", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), `{"changes":{"{file}":[{"range":{"start":{"line":10,`+
				`"character":0},"end":{"line":10,"character":0}},"newText":"// end\n"}]}}`)
			assert.NoError(t, err, "Plan of an edit after the last line")
			assert.Equal(t, applied(t, lsptest.Content, got.Items), lsptest.Content+"// end\n",
				"a.fake after the edit")
		})

		t.Run("refuses an edit past the end of the file", func(t *testing.T) {
			t.Parallel()
			_, err := renamedAt(t, sample(), `{"changes":{"{file}":[{"range":{"start":{"line":12,`+
				`"character":0},"end":{"line":12,"character":0}},"newText":"// end\n"}]}}`)
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("keeps the carriage return after the end of a line", func(t *testing.T) {
			t.Parallel()
			crlf := strings.ReplaceAll(lsptest.Content, "\n", "\r\n")
			got, err := renamedAt(t, map[string]string{"a.fake": crlf}, `{"changes":{"{file}":[`+
				`{"range":{"start":{"line":2,"character":5},"end":{"line":2,"character":99}},`+
				`"newText":"Vault struct {"}]}}`)
			assert.NoError(t, err, "Plan of an edit to the end of line 2")
			assert.Equal(t, applied(t, crlf, got.Items), strings.Replace(crlf, "Store struct {", "Vault struct {", 1),
				"a.fake after the edit")
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("converts the positions of a line of 20000 declarations within a second", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Minified, map[string]string{"a.fake": lsptest.Bundle(20000)})
			request, of := engine.Request{Scope: "a.fake"}, declared("F0", sema.KindFunction)
			// The first question starts the server and waits for it to settle.
			_, err := e.Relate(t.Context(), request, of, sema.ReferencedBy)
			assert.NoError(t, err, "the first Relate of the uses of F0")

			start := time.Now()
			got, err := e.Relate(t.Context(), request, of, sema.ReferencedBy)
			took := time.Since(start)
			assert.NoError(t, err, "the second Relate of the uses of F0")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the declarations that use F0")
			assert.True(t, took < time.Second, "the second Relate takes less than a second: "+took.String())
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("converts a UTF-16 range to bytes", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Unicode, emoji).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Offset: 30})
			assert.NoError(t, err, "Resolve of Störe")
			assert.Length(t, got.Items, 1, "the declarations that Störe denotes")
			assert.Equal(t, got.Items[0].Snippet, "Störe", "the snippet of Störe")
			assert.Equal(t, got.Items[0].Span.Start.Offset, 30, "the offset of Störe")
		})

		t.Run("converts an offset on the first line", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Pointed, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Offset: 8})
			assert.NoError(t, err, "Resolve at byte 8")
			assert.Equal(t, names(got.Items), []string{"a"}, "the declaration at byte 8")
		})

		t.Run("converts an offset at the start of a line", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Pointed, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Offset: 89})
			assert.NoError(t, err, "Resolve at byte 89")
			assert.Equal(t, names(got.Items), []string{"After"}, "the declaration at byte 89")
		})

		t.Run("converts an offset on the last line", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Pointed, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, source.Position{Offset: 94})
			assert.NoError(t, err, "Resolve at byte 94")
			assert.Equal(t, names(got.Items), []string{"After"}, "the declaration at byte 94")
		})
	})
}
