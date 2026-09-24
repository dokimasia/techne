// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/test/corpus"
	"go.dokimi.dev/techne/tool"
)

// content is a Go file of one struct with one field, and one function.
const content = "package a\n\ntype Store struct {\n\tsize int\n}\n\nfunc Get() int { return 1 }\n"

// declared returns the declaration of name at the bytes from to end of
// content, as the outline tool returns it at the detail source.
func declared(name string, from, end int, members ...tool.Declaration) tool.Declaration {
	line, column := corpus.Position([]byte(content), from)
	endLine, endColumn := corpus.Position([]byte(content), end)
	return tool.Declaration{
		Name: name, Kind: sema.KindFunction, Line: line + 1, Snippet: content[from:end],
		Span: &source.Span{
			Path:  "a.go",
			Start: source.Position{Offset: from, Line: line, Column: column},
			End:   source.Position{Offset: end, Line: endLine, Column: endColumn},
		},
		Members: members,
	}
}

// The bytes of the declarations of content: Store from 11 to 42, its field
// size from 32 to 40, and Get from 44 to 71.
const (
	storeFrom, storeEnd = 11, 42
	sizeFrom, sizeEnd   = 32, 40
	getFrom, getEnd     = 44, 71
)

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Spans", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no problem for declarations that match their file", func(t *testing.T) {
			t.Parallel()
			fitting := declared("Store", storeFrom, storeEnd, declared("size", sizeFrom, sizeEnd))
			got := corpus.Spans([]byte(content), []tool.Declaration{fitting, declared("Get", getFrom, getEnd)})
			assert.Empty(t, got, "the problems of fitting declarations")
		})

		tests := []struct {
			name   string
			change func(*tool.Declaration)
		}{
			{
				name:   "reports a declaration without a span",
				change: func(d *tool.Declaration) { d.Span = nil },
			},
			{
				name:   "reports a span past the end of the file",
				change: func(d *tool.Declaration) { d.Span.End.Offset = 1000 },
			},
			{
				name:   "reports a snippet that differs from its span",
				change: func(d *tool.Declaration) { d.Snippet += " " },
			},
			{
				name:   "reports a snippet without the name",
				change: func(d *tool.Declaration) { d.Name = "Other" },
			},
			{
				name:   "reports a line that differs from the span",
				change: func(d *tool.Declaration) { d.Line++ },
			},
			{
				name:   "reports a column that differs from the offset",
				change: func(d *tool.Declaration) { d.Span.Start.Column++ },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				d := declared("Get", getFrom, getEnd)
				tt.change(&d)
				got := corpus.Spans([]byte(content), []tool.Declaration{d})
				assert.NotEmpty(t, got, "the problems of the declaration")
			})
		}

		t.Run("reports a member outside its container", func(t *testing.T) {
			t.Parallel()
			header := declared("Store", storeFrom, 30, declared("size", sizeFrom, sizeEnd))
			assert.NotEmpty(t, corpus.Spans([]byte(content), []tool.Declaration{header}),
				"the problems of a member that leaves Store")
		})
	})

	t.Run("Position", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the line and column of an offset", func(t *testing.T) {
			t.Parallel()
			line, column := corpus.Position([]byte(content), sizeFrom)
			assert.Equal(t, [2]int{line, column}, [2]int{3, 1}, "the position of size")
		})

		t.Run("returns the first line for an offset of zero", func(t *testing.T) {
			t.Parallel()
			line, column := corpus.Position([]byte(content), 0)
			assert.Equal(t, [2]int{line, column}, [2]int{0, 0}, "the position of offset 0")
		})
	})

	t.Run("Ranked", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no problem for an exact match first", func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, corpus.Ranked("Get", []tool.Declaration{{Name: "Get"}, {Name: "GetAll"}}), "the problems")
		})

		t.Run("reports another name first", func(t *testing.T) {
			t.Parallel()
			got := corpus.Ranked("Get", []tool.Declaration{{Name: "GetAll"}, {Name: "Get"}})
			assert.NotEmpty(t, got, "the problems")
		})

		t.Run("reports an empty result", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, corpus.Ranked("Get", nil), "the problems of no result")
		})
	})

	t.Run("Sites", func(t *testing.T) {
		t.Parallel()
		files := map[string]string{
			"b.go": "package b\n\nfunc use() { a.Get() }\n",
			"c.js": "class Get {\n  static init() { return this; }\n  static thisway() {}\n}\n",
			"d.rs": "impl Get {\n    fn new() -> Self { Self {} }\n}\n",
		}
		read := func(p string) ([]byte, error) {
			if text, found := files[p]; found {
				return []byte(text), nil
			}
			return nil, errors.New("no such file")
		}

		tests := []struct {
			name  string
			edge  tool.Connected
			clean bool
		}{
			{
				name:  "returns no problem for a site that shows the name",
				edge:  tool.Connected{Name: "use", Path: "b.go", Line: 3, Via: "func use() { a.Get() }"},
				clean: true,
			},
			{
				name: "reports a site that shows neither name",
				edge: tool.Connected{Name: "other", Path: "b.go", Line: 1},
			},
			{
				name:  "returns no problem for a site that shows a receiver keyword",
				edge:  tool.Connected{Name: "other", Path: "c.js", Line: 2},
				clean: true,
			},
			{
				name: "reports a site whose keyword is part of a longer name",
				edge: tool.Connected{Name: "other", Path: "c.js", Line: 3},
			},
			{
				name:  "returns no problem for a site that shows Self in an impl",
				edge:  tool.Connected{Name: "other", Path: "d.rs", Line: 2},
				clean: true,
			},
			{
				name: "reports a via that differs from the line",
				edge: tool.Connected{Name: "use", Path: "b.go", Line: 3, Via: "func use() {}"},
			},
			{
				name: "reports a line past the end of the file",
				edge: tool.Connected{Name: "use", Path: "b.go", Line: 9},
			},
			{
				name: "reports a file that cannot be read",
				edge: tool.Connected{Name: "use", Path: "e.go", Line: 1},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				problems := corpus.Sites("Get", []tool.Connected{tt.edge}, read)
				assert.Equal(t, len(problems) == 0, tt.clean, "the problems of the site")
			})
		}
	})

	t.Run("Line", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a line counted from one", func(t *testing.T) {
			t.Parallel()
			got, found := corpus.Line([]byte(content), 3)
			assert.True(t, found, "the presence of line 3")
			assert.Equal(t, got, "type Store struct {", "line 3")
		})

		t.Run("returns false past the last line", func(t *testing.T) {
			t.Parallel()
			_, found := corpus.Line([]byte(content), 20)
			assert.False(t, found, "the presence of line 20")
		})

		t.Run("drops a carriage return", func(t *testing.T) {
			t.Parallel()
			got, _ := corpus.Line([]byte("a\r\nb\r\n"), 1)
			assert.Equal(t, got, "a", "line 1")
		})
	})
}
