// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestIndentation(t *testing.T) {
	t.Parallel()

	t.Run("Indentation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want lang.Indent
		}{
			{
				name: "returns tabs for a file indented with tabs",
				give: "func f() {\n\tif ok {\n\t\treturn\n\t}\n}\n",
				want: lang.Indent{Spaces: false, Width: 4},
			},
			{
				name: "returns two spaces for a file indented with two spaces",
				give: "class Store {\n  get() {\n    return 1;\n  }\n}\n",
				want: lang.Indent{Spaces: true, Width: 2},
			},
			{
				name: "returns four spaces for a file indented with four spaces",
				give: "class Store {\n    int get() {\n        return 1;\n    }\n}\n",
				want: lang.Indent{Spaces: true, Width: 4},
			},
			{
				name: "returns four spaces for a file without indentation",
				give: "a\nb\n",
				want: lang.Indent{Spaces: true, Width: 4},
			},
			{
				name: "returns spaces for a file with as many lines indented by tabs as by spaces",
				give: "a\n\tb\nc\n  d\n",
				want: lang.Indent{Spaces: true, Width: 2},
			},
			{
				name: "returns two spaces at two thirds as many changes as four",
				give: "a\n  b\na\n    c\na\n    c\n",
				want: lang.Indent{Spaces: true, Width: 2},
			},
			{
				name: "returns four spaces at fewer than two thirds as many changes of two",
				give: "a\n  b\na\n    c\na\n    c\na\n    c\n",
				want: lang.Indent{Spaces: true, Width: 4},
			},
			{
				name: "skips a line that contains only white space",
				give: "a {\n  b\n        \n  c\n        \n  d\n}\n",
				want: lang.Indent{Spaces: true, Width: 2},
			},
			{
				name: "skips the line endings of a CRLF file",
				give: "a {\r\n  b\r\n        \r\n  c\r\n        \r\n  d\r\n}\r\n",
				want: lang.Indent{Spaces: true, Width: 2},
			},
			{
				name: "skips a line aligned under a word of a line that ends with a comma",
				give: "const first = 1,\n      second = 2;\nconst third = 3,\n      fourth = 4;\n" +
					"function f() {\n  return first;\n}\n",
				want: lang.Indent{Spaces: true, Width: 2},
			},
			{
				name: "reads no line after the first 10,000",
				give: strings.Repeat("x\n", 10000) + "\ty\n\tz\n",
				want: lang.Indent{Spaces: true, Width: 4},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Indentation([]byte(tt.give)), tt.want, "the indentation")
			})
		}
	})
}
