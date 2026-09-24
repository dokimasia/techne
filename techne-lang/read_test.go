// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestRead(t *testing.T) {
	t.Parallel()

	t.Run("Readable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a file of Largest bytes", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"src/a.ts": {Data: bytes.Repeat([]byte("x"), lang.Largest)}}
			assert.NoError(t, lang.Readable(fsys, "src/a.ts"), "Readable")
		})

		t.Run("returns LargeError for a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"src/a.ts": {Data: bytes.Repeat([]byte("x"), lang.Largest+1)}}
			large, ok := errors.AsType[lang.LargeError](lang.Readable(fsys, "src/a.ts"))
			assert.True(t, ok, "LargeError")
			assert.Equal(t, large, lang.LargeError{Path: "src/a.ts", Size: lang.Largest + 1}, "error")
		})

		t.Run("returns GeneratedError for a file the .gitignore excludes", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				".gitignore":    {Data: []byte("dist\n")},
				"web/dist/a.ts": {Data: []byte("")},
			}
			_, ok := errors.AsType[lang.GeneratedError](lang.Readable(fsys, "web/dist/a.ts"))
			assert.True(t, ok, "GeneratedError")
		})

		t.Run("returns nil for a file beside an excluded directory", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				".gitignore":    {Data: []byte("dist\n")},
				"web/dist/a.ts": {Data: []byte("")},
				"web/src/b.ts":  {Data: []byte("")},
			}
			assert.NoError(t, lang.Readable(fsys, "web/src/b.ts"), "Readable")
		})

		t.Run("applies the .gitignore of a directory below that directory only", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				"one/.gitignore":     {Data: []byte("generated\n")},
				"one/generated/a.ts": {Data: []byte("")},
				"two/generated/b.ts": {Data: []byte("")},
			}
			assert.HasError(t, lang.Readable(fsys, "one/generated/a.ts"), "one")
			assert.NoError(t, lang.Readable(fsys, "two/generated/b.ts"), "two")
		})

		t.Run("returns GeneratedError under an excluded directory that a pattern re-includes", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				".gitignore":   {Data: []byte("dist/\n!dist/keep.ts\n")},
				"dist/keep.ts": {Data: []byte("")},
			}
			_, ok := errors.AsType[lang.GeneratedError](lang.Readable(fsys, "dist/keep.ts"))
			assert.True(t, ok, "GeneratedError")
		})

		t.Run("returns nil for a directory", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"src/a.ts": {Data: []byte("")}}
			assert.NoError(t, lang.Readable(fsys, "src"), "Readable")
		})

		t.Run("returns another error for a path that does not exist", func(t *testing.T) {
			t.Parallel()
			err := lang.Readable(fstest.MapFS{}, "nowhere.ts")
			_, generated := errors.AsType[lang.GeneratedError](err)
			_, large := errors.AsType[lang.LargeError](err)
			assert.HasError(t, err, "Readable")
			assert.False(t, generated || large, "error type")
		})
	})

	t.Run("Large", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for Largest bytes", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, lang.Large("a.ts", lang.Largest), "Large")
		})

		t.Run("returns LargeError above Largest", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lang.Large("a.ts", lang.Largest+1),
				error(lang.LargeError{Path: "a.ts", Size: lang.Largest + 1}), "Large")
		})
	})

	t.Run("LargeError.Error", func(t *testing.T) {
		t.Parallel()

		t.Run("names the path and the size", func(t *testing.T) {
			t.Parallel()
			got := lang.LargeError{Path: "dist/app.js", Size: 3_400_000}.Error()
			assert.Equal(t, got, "lang: dist/app.js is 3400000 bytes, larger than the 1048576 bytes an engine reads",
				"message")
		})
	})

	t.Run("LineAt", func(t *testing.T) {
		t.Parallel()

		short := []byte("one\ntwo\r\n\nfour")
		long := strings.Repeat("a", 500) + "NAME" + strings.Repeat("b", 496)
		tests := []struct {
			name    string
			content string
			give    int
			want    string
		}{
			{name: "returns the line that contains the offset", content: string(short), give: 5, want: "two"},
			{name: "returns the first line for offset zero", content: string(short), give: 0, want: "one"},
			{
				name: "returns the line that a newline ends for the offset of the newline", content: string(short),
				give: 3, want: "one",
			},
			{name: "removes the carriage return of a CRLF line", content: string(short), give: 4, want: "two"},
			{name: "returns an empty string for an empty line", content: string(short), give: 9, want: ""},
			{
				name: "returns the last line for the offset at the end", content: string(short),
				give: len(short), want: "four",
			},
			{name: "returns an empty string for a negative offset", content: string(short), give: -1, want: ""},
			{
				name: "returns an empty string for an offset past the end", content: string(short),
				give: len(short) + 1, want: "",
			},
			{
				name: "cuts a long line around the offset", content: "one\n" + long + "\n",
				give: 504, want: "…" + long[440:680] + "…",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.LineAt([]byte(tt.content), tt.give), tt.want, "LineAt")
			})
		}
	})

	t.Run("Excerpt", func(t *testing.T) {
		t.Parallel()

		long := strings.Repeat("a", 500) + "NAME" + strings.Repeat("b", 496)
		wide := "x" + strings.Repeat("é", 500)
		spaced := strings.Repeat("a", 200) + strings.Repeat(" ", 100) + strings.Repeat("b", 700)
		tests := []struct {
			name   string
			line   string
			column int
			want   string
		}{
			{
				name: "returns a line of LineLimit bytes whole", line: strings.Repeat("a", lang.LineLimit),
				column: 7, want: strings.Repeat("a", lang.LineLimit),
			},
			{
				name: "cuts a long line to a window around the column", line: long,
				column: 500, want: "…" + long[440:680] + "…",
			},
			{
				name: "starts the window at the start of the line for a column near it", line: long,
				column: 10, want: long[:lang.LineLimit] + "…",
			},
			{
				name: "ends the window at the end of the line for a column near it", line: long,
				column: 990, want: "…" + long[1000-lang.LineLimit:],
			},
			{
				name: "cuts a long line at a rune boundary", line: wide,
				column: 0, want: "x" + strings.Repeat("é", 119) + "…",
			},
			{
				name: "drops the white space at a cut", line: spaced,
				column: 0, want: strings.Repeat("a", 200) + "…",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := lang.Excerpt([]byte(tt.line), tt.column)
				assert.Equal(t, got, tt.want, "Excerpt")
				assert.True(t, utf8.ValidString(got), "Excerpt returns valid UTF-8")
			})
		}
	})

	t.Run("Clipped", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			text  string
			limit int
			want  string
		}{
			{name: "returns a text within the limit unchanged", text: "abc", limit: 3, want: "abc"},
			{name: "cuts a longer text with an ellipsis", text: "abcdef", limit: 3, want: "abc…"},
			{name: "cuts before a rune that crosses the limit", text: "abé", limit: 3, want: "ab…"},
			{name: "drops the white space before the cut", text: "ab  cd", limit: 4, want: "ab…"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Clipped(tt.text, tt.limit), tt.want, "Clipped")
			})
		}
	})
}
