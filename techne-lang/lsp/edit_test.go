// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// The JSON of the ranges and edits that the workspace edits of these cases contain.
const (
	storeName = `{"start":{"line":2,"character":5},"end":{"line":2,"character":10}}`
	vault     = `{"range":` + storeName + `,"newText":"Vault"}`
)

// documentEdit returns the JSON of a TextDocumentEdit of the document at uri.
func documentEdit(uri string, edits ...string) string {
	return fmt.Sprintf(`{"textDocument":{"uri":%q,"version":null},"edits":[%s]}`, uri, strings.Join(edits, ","))
}

// ordered returns the JSON of a workspace edit with documentChanges.
func ordered(changes ...string) string {
	return `{"documentChanges":[` + strings.Join(changes, ",") + `]}`
}

// insert returns the JSON of an edit that inserts text at a line and a character.
func insert(line, character int, text string) string {
	return fmt.Sprintf(`{"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"newText":%q}`,
		line, character, line, character, text)
}

func TestEdit(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a map of edits", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Default, sample()))
			assert.NoError(t, err, "Plan of a rename")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit}, "the changes of the plan")
			assert.Length(t, got.Items[0].Edits, 2, "the edits of a.fake")
		})

		t.Run("sorts the edits of a file by offset", func(t *testing.T) {
			t.Parallel()
			got, err := renameWith(t, serving(t, lsptest.Default, sample()))
			assert.NoError(t, err, "Plan of a rename")
			assert.Pairwise(t, got.Items[0].Edits, func(earlier, later edit.TextEdit) bool {
				return earlier.Span.Start.Offset < later.Span.Start.Offset
			}, "the start offsets of the edits of a.fake")
		})

		t.Run("refuses overlapping edits", func(t *testing.T) {
			t.Parallel()
			_, err := renamedAt(t, sample(), `{"changes":{"{file}":[`+
				`{"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":10}},"newText":"type Vault"},`+
				vault+`]}}`)
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "overlapping", "the error of Plan")
		})

		t.Run("reads documentChanges in place of the map", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), `{"changes":{"{file}":[{"range":`+storeName+
				`,"newText":"Wrong"}]},"documentChanges":[`+documentEdit("{file}", vault)+`]}`)
			assert.NoError(t, err, "Plan of an edit with both forms")
			assert.Equal(t, got.Items[0].Edits[0].New, "Vault", "the text of the edit")
		})

		t.Run("moves a renamed file after its edits", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(documentEdit("{file}", vault),
				`{"kind":"rename","oldUri":"{file}","newUri":"{root}/vault.fake"}`))
			assert.NoError(t, err, "Plan of an edit and a rename")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit, edit.ChangeMove},
				"the changes of the plan")
			assert.Equal(t, got.Items[1].To, source.Path("vault.fake"), "the destination of the move")
		})

		t.Run("addresses an edit of a renamed file to its old path", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(
				`{"kind":"rename","oldUri":"{file}","newUri":"{root}/vault.fake"}`,
				documentEdit("{root}/vault.fake", vault)))
			assert.NoError(t, err, "Plan of a rename and an edit")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit, edit.ChangeMove},
				"the changes of the plan")
			assert.Equal(t, paths(got.Items), []source.Path{"a.fake", "a.fake"}, "the paths of the changes")
		})

		t.Run("folds the edits of a created file into its content", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(`{"kind":"create","uri":"{root}/new.fake"}`,
				documentEdit("{root}/new.fake", insert(0, 0, "package a\n"))))
			assert.NoError(t, err, "Plan of a create and an edit")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeCreate}, "the changes of the plan")
			assert.Equal(t, got.Items[0].Path, source.Path("new.fake"), "the path of the created file")
			assert.Equal(t, string(got.Items[0].Content), "package a\n", "the content of the created file")
		})

		t.Run("creates an empty file with content that is not nil", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(`{"kind":"create","uri":"{root}/new.fake"}`))
			assert.NoError(t, err, "Plan of a create")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeCreate}, "the changes of the plan")
			assert.True(t, got.Items[0].Content != nil, "the content of the created file is not nil")
			assert.Empty(t, got.Items[0].Content, "the content of the created file")
		})

		t.Run("keeps an existing file for a create that ignores it", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(
				`{"kind":"create","uri":"{file}","options":{"ignoreIfExists":true}}`,
				documentEdit("{file}", vault)))
			assert.NoError(t, err, "Plan of a create that ignores a.fake")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeEdit}, "the changes of the plan")
		})

		t.Run("refuses a create of an existing file", func(t *testing.T) {
			t.Parallel()
			_, err := renamedAt(t, sample(), ordered(`{"kind":"create","uri":"{file}"}`))
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("composes two edit lists of one file", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(
				documentEdit("{file}", insert(0, 0, "// renamed\n")),
				documentEdit("{file}", `{"range":{"start":{"line":3,"character":5},`+
					`"end":{"line":3,"character":10}},"newText":"Vault"}`)))
			assert.NoError(t, err, "Plan of two edit lists")
			want := "// renamed\n" + strings.Replace(lsptest.Content, "type Store", "type Vault", 1)
			assert.Equal(t, applied(t, lsptest.Content, got.Items), want, "a.fake after the plan")
		})

		t.Run("keeps the order of inserts at one offset", func(t *testing.T) {
			t.Parallel()
			edits := []string{insert(8, 5, "Late")}
			var want strings.Builder
			for i := range 20 {
				edits = append(edits, insert(0, 0, fmt.Sprintf("%d\n", i)))
				fmt.Fprintf(&want, "%d\n", i)
			}
			got, err := renamedAt(t, sample(), `{"changes":{"{file}":[`+strings.Join(edits, ",")+`]}}`)
			assert.NoError(t, err, "Plan of twenty inserts at one offset")
			assert.HasPrefix(t, applied(t, lsptest.Content, got.Items), want.String(), "a.fake after the plan")
		})

		t.Run("deletes a file without its edits", func(t *testing.T) {
			t.Parallel()
			got, err := renamedAt(t, sample(), ordered(documentEdit("{file}", vault),
				`{"kind":"delete","uri":"{file}"}`))
			assert.NoError(t, err, "Plan of an edit and a delete")
			assert.Equal(t, kinds(got.Items), []edit.ChangeKind{edit.ChangeDelete}, "the changes of the plan")
		})

		t.Run("refuses a rename of a directory", func(t *testing.T) {
			t.Parallel()
			_, err := renamedAt(t, map[string]string{"a.fake": lsptest.Content, "pkg/x.fake": lsptest.Content},
				ordered(`{"kind":"rename","oldUri":"{root}/pkg","newUri":"{root}/lib"}`))
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "directory", "the error of Plan")
		})
	})
}
