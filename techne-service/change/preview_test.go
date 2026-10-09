// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// The contents of a.fx with three lines, and with the crlf line endings of [original].
const (
	threeLines = "one\ntwo\nthree\n"
	crlf       = "one\r\ntwo\r\n"
)

func TestPreview(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the text that an edit writes with its file", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Rewrites, []edit.Rewrite{{Path: "a.fx", Line: 1, Now: "// Doc."}},
				"the rewrites of the outcome")
		})

		t.Run("returns the text that an edit replaces", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{changes: []edit.Change{replacing("a.fx", 4, 7, "three")}}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Rewrites, []edit.Rewrite{{Path: "a.fx", Line: 2, Was: "two", Now: "three"}},
				"the rewrites of the outcome")
		})

		tests := []struct {
			name    string
			content string
			give    []edit.TextEdit
			want    []edit.Rewrite
		}{
			{
				name: "returns one rewrite for the edits of one line",
				give: []edit.TextEdit{replaced(0, 1, "O"), replaced(2, 3, "E")},
				want: []edit.Rewrite{{Path: "a.fx", Line: 1, Was: "one", Now: "OnE"}},
			},
			{
				name: "returns one rewrite for the edits of adjacent lines",
				give: []edit.TextEdit{replaced(0, 3, "1"), replaced(4, 7, "2")},
				want: []edit.Rewrite{{Path: "a.fx", Line: 1, Was: "one\ntwo", Now: "1\n2"}},
			},
			{
				name:    "returns a rewrite for each run of lines",
				content: threeLines,
				give:    []edit.TextEdit{replaced(0, 3, "1"), replaced(8, 13, "3")},
				want: []edit.Rewrite{
					{Path: "a.fx", Line: 1, Was: "one", Now: "1"},
					{Path: "a.fx", Line: 3, Was: "three", Now: "3"},
				},
			},
			{
				name:    "returns the lines after the lines that an earlier edit adds",
				content: threeLines,
				give:    []edit.TextEdit{replaced(0, 0, "zero\n"), replaced(8, 13, "3")},
				want: []edit.Rewrite{
					{Path: "a.fx", Line: 1, Now: "zero"},
					{Path: "a.fx", Line: 3, Was: "three", Now: "3"},
				},
			},
			{
				name: "returns the line that an edit adds before two lines that the edits keep",
				give: []edit.TextEdit{replaced(0, 0, "zero\n"), replaced(4, 7, "two")},
				want: []edit.Rewrite{{Path: "a.fx", Line: 1, Now: "zero"}},
			},
			{
				name: "returns the line that an edit adds at the end of a line",
				give: []edit.TextEdit{replaced(3, 3, "\nmid")},
				want: []edit.Rewrite{{Path: "a.fx", Line: 2, Now: "mid"}},
			},
			{
				name: "returns the line that an edit removes",
				give: []edit.TextEdit{replaced(4, 8, "")},
				want: []edit.Rewrite{{Path: "a.fx", Line: 2, Was: "two"}},
			},
			{
				name: "returns no rewrite for an edit that keeps its line",
				give: []edit.TextEdit{replaced(4, 7, "two")},
			},
			{
				name:    "returns the lines of a crlf file without the carriage return",
				content: crlf,
				give:    []edit.TextEdit{replaced(5, 8, "three")},
				want:    []edit.Rewrite{{Path: "a.fx", Line: 2, Was: "two", Now: "three"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				files, s := serving(t, planner{changes: []edit.Change{{
					Kind: edit.ChangeEdit, Path: "a.fx", Edits: tt.give,
				}}}, clean())
				if tt.content != "" {
					files.content["a.fx"] = tt.content
				}
				got, err := s.Apply(t.Context(), asking(true))
				assert.NoError(t, err, "Apply")
				assert.Equal(t, got.Rewrites, tt.want, "the rewrites of the outcome")
			})
		}

		t.Run("returns the rewrite of a file that the plan also moves", func(t *testing.T) {
			t.Parallel()
			moved := []edit.Change{
				replacing("a.fx", 4, 7, "three"),
				{Kind: edit.ChangeMove, Path: "a.fx", To: "b.fx"},
			}
			_, s := serving(t, planner{changes: moved}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Rewrites, []edit.Rewrite{{Path: "a.fx", Line: 2, Was: "two", Now: "three"}},
				"the rewrites of the outcome")
		})

		t.Run("returns the rewrites of a refused change", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, marking("// Doc."))
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.False(t, got.Applied, "the application of the change")
			assert.Length(t, got.Rewrites, 1, "the rewrites of the outcome")
		})

		t.Run("returns no rewrite for a created file", func(t *testing.T) {
			t.Parallel()
			made := []edit.Change{{Kind: edit.ChangeCreate, Path: "new.fx", Content: []byte("made\n")}}
			_, s := serving(t, planner{changes: made}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Empty(t, got.Rewrites, "the rewrites of the outcome")
			assert.Equal(t, got.Changes, made, "the changes of the outcome")
		})
	})
}

// replaced returns the edit of a.fx that replaces the bytes from start to end with text.
func replaced(start, end int, text string) edit.TextEdit {
	return edit.TextEdit{
		Span: source.Span{Path: "a.fx", Start: source.Position{Offset: start}, End: source.Position{Offset: end}},
		New:  text,
	}
}
