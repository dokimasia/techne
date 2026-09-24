// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// reading returns the read function of [lang.Lowered] over files, which are lines by path. A
// path that files does not contain fails with [fs.ErrNotExist].
func reading(files map[source.Path][]string) func(source.Path) (func(int) string, error) {
	return func(p source.Path) (func(int) string, error) {
		lines, known := files[p]
		if !known {
			return nil, fs.ErrNotExist
		}
		return func(n int) string { return lines[n] }, nil
	}
}

// broken returns the caveat of [lang.Lowered] from the server for sites, listed as the note lists
// them, in the files paths.
func broken(sites string, paths ...source.Path) []trust.Caveat {
	return []trust.Caveat{{
		Code: trust.CaveatBuildBroken,
		Note: "the server reports an error that can hide a use at " + sites + ". Names are bound " +
			"where the server could bind them and matched by text elsewhere",
		Paths: paths,
	}}
}

func TestLowered(t *testing.T) {
	t.Parallel()

	t.Run("Spanned", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every line from the start to the end of a span", func(t *testing.T) {
			t.Parallel()
			got := lang.Spanned(source.Span{
				Path:  "a.go",
				Start: source.Position{Line: 2},
				End:   source.Position{Line: 4},
			})
			assert.Equal(t, got, lang.Lines{"a.go": {2: true, 3: true, 4: true}}, "Spanned")
		})

		t.Run("returns the start line of a span that ends before it", func(t *testing.T) {
			t.Parallel()
			got := lang.Spanned(source.Span{Path: "a.go", Start: source.Position{Line: 7}})
			assert.Equal(t, got, lang.Lines{"a.go": {7: true}}, "Spanned")
		})

		t.Run("returns the lines of each path by path", func(t *testing.T) {
			t.Parallel()
			got := lang.Spanned(
				source.Span{Path: "a.go", Start: source.Position{Line: 1}, End: source.Position{Line: 1}},
				source.Span{Path: "b.go", Start: source.Position{Line: 5}, End: source.Position{Line: 5}},
				source.Span{Path: "a.go", Start: source.Position{Line: 9}, End: source.Position{Line: 9}},
			)
			assert.Equal(t, got, lang.Lines{"a.go": {1: true, 9: true}, "b.go": {5: true}}, "Spanned")
		})
	})

	t.Run("Writing", func(t *testing.T) {
		t.Parallel()

		risky := lang.Writing("Get", lang.Lines{"a.go": {3: true}})
		tests := []struct {
			name string
			path source.Path
			line int
			text string
			want bool
		}{
			{
				name: "reports a line outside the sites that writes the name",
				path: "a.go", line: 5, text: "n := s.Get()", want: true,
			},
			{name: "leaves out a line of a site", path: "a.go", line: 3, text: "n := s.Get()"},
			{name: "leaves out a line that writes a longer identifier", path: "a.go", line: 5, text: "s.GetAll()"},
			{name: "leaves out a line without the name", path: "a.go", line: 5, text: "return nil"},
			{
				name: "reports the line of a site in another file",
				path: "b.go", line: 3, text: "n := s.Get()", want: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, risky(tt.path, tt.line, tt.text), tt.want, "the rule of Writing")
			})
		}

		t.Run("reports every line for an empty name", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lang.Writing("", lang.Lines{"a.go": {3: true}})("a.go", 3, ""), "the rule of Writing")
		})
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		risky := lang.Binding("a.go", 4, true)
		tests := []struct {
			name string
			path source.Path
			line int
			want bool
		}{
			{name: "reports the line of the definition", path: "a.go", line: 4, want: true},
			{name: "leaves out another line of the file", path: "a.go", line: 5},
			{name: "leaves out the line in another file", path: "b.go", line: 4},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, risky(tt.path, tt.line, "x"), tt.want, "the rule of Binding")
			})
		}

		t.Run("reports every line when nothing is found", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lang.Binding("a.go", 4, false)("b.go", 9, "x"), "the rule of Binding")
		})
	})

	t.Run("Everywhere", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a line without text", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lang.Everywhere("a.go", 0, ""), "Everywhere")
		})
	})

	t.Run("Nowhere", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves out a line that writes a name", func(t *testing.T) {
			t.Parallel()
			assert.False(t, lang.Nowhere("a.go", 0, "Get()"), "Nowhere")
		})
	})

	t.Run("Lowered", func(t *testing.T) {
		t.Parallel()

		files := map[source.Path][]string{
			"a.go": {"package a", "n := s.Get()", "return nil", "m := s.Get()", "s.GetAll()", "", "", ""},
			"b.go": {"package a", "", "k := s.Get()"},
		}

		t.Run("returns no tier without errors", func(t *testing.T) {
			t.Parallel()
			tier, caveats := lang.Lowered(nil, reading(files), lang.Everywhere, "the server")
			assert.Equal(t, tier, trust.None, "the tier")
			assert.Empty(t, caveats, "the caveats")
		})

		t.Run("returns no tier for errors that the rule leaves out", func(t *testing.T) {
			t.Parallel()
			errors := map[source.Path][]int{"a.go": {1, 3}}
			tier, caveats := lang.Lowered(errors, reading(files), lang.Nowhere, "the server")
			assert.Equal(t, tier, trust.None, "the tier")
			assert.Empty(t, caveats, "the caveats")
		})

		t.Run("returns Indexed for the errors that the rule reports", func(t *testing.T) {
			t.Parallel()
			errors := map[source.Path][]int{"a.go": {1, 2, 4}, "b.go": {2}}
			risky := lang.Writing("Get", lang.Lines{"a.go": {3: true}})
			tier, caveats := lang.Lowered(errors, reading(files), risky, "the server")
			assert.Equal(t, tier, trust.Indexed, "the tier")
			assert.Equal(t, caveats, broken("a.go:2, b.go:3", "a.go", "b.go"), "the caveats")
		})

		t.Run("sorts the lines by path then by line", func(t *testing.T) {
			t.Parallel()
			errors := map[source.Path][]int{"b.go": {2, 0}, "a.go": {3, 1}}
			_, caveats := lang.Lowered(errors, reading(files), lang.Everywhere, "the server")
			assert.Equal(t, caveats, broken("a.go:2, a.go:4, b.go:1, b.go:3", "a.go", "b.go"), "the caveats")
		})

		t.Run("lists a line with two errors once", func(t *testing.T) {
			t.Parallel()
			errors := map[source.Path][]int{"a.go": {3, 3}}
			_, caveats := lang.Lowered(errors, reading(files), lang.Everywhere, "the server")
			assert.Equal(t, caveats, broken("a.go:4", "a.go"), "the caveats")
		})

		t.Run("counts the lines past the fifth", func(t *testing.T) {
			t.Parallel()
			errors := map[source.Path][]int{"a.go": {0, 1, 2, 3, 4, 5, 6}}
			_, caveats := lang.Lowered(errors, reading(files), lang.Everywhere, "the server")
			assert.Equal(t, caveats, broken("a.go:1, a.go:2, a.go:3, a.go:4, a.go:5 and 2 more", "a.go"),
				"the caveats")
		})

		t.Run("reports an error in a file that it cannot read", func(t *testing.T) {
			t.Parallel()
			errors := map[source.Path][]int{"gone.go": {0}}
			tier, caveats := lang.Lowered(errors, reading(files), lang.Nowhere, "the server")
			assert.Equal(t, tier, trust.Indexed, "the tier")
			assert.Equal(t, caveats, broken("gone.go:1", "gone.go"), "the caveats")
		})
	})
}
